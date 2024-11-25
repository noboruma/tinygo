package compiler

// This file lowers channel operations (make/send/recv/close) to runtime calls
// or pseudo-operations that are lowered during goroutine lowering.

import (
	"go/types"

	"github.com/tinygo-org/tinygo/compiler/llvmutil"
	"golang.org/x/tools/go/ssa"
	"tinygo.org/x/go-llvm"
)

func (b *builder) createMakeChan(expr *ssa.MakeChan) llvm.Value {
	elementSize := b.targetData.TypeAllocSize(b.getLLVMType(expr.Type().Underlying().(*types.Chan).Elem()))
	elementSizeValue := llvm.ConstInt(b.uintptrType, elementSize, false)
	bufSize := b.getValue(expr.Size, getPos(expr))
	b.createChanBoundsCheck(elementSize, bufSize, expr.Size.Type().Underlying().(*types.Basic), expr.Pos())
	if bufSize.Type().IntTypeWidth() < b.uintptrType.IntTypeWidth() {
		bufSize = b.CreateZExt(bufSize, b.uintptrType, "")
	} else if bufSize.Type().IntTypeWidth() > b.uintptrType.IntTypeWidth() {
		bufSize = b.CreateTrunc(bufSize, b.uintptrType, "")
	}
	return b.createRuntimeCall("chanMake", []llvm.Value{elementSizeValue, bufSize}, "")
}

// createChanSend emits a pseudo chan send operation. It is lowered to the
// actual channel send operation during goroutine lowering.
func (b *builder) createChanSend(instr *ssa.Send) {
	ch := b.getValue(instr.Chan, getPos(instr))
	chanValue := b.getValue(instr.X, getPos(instr))

	// store value-to-send
	valueType := b.getLLVMType(instr.X.Type())
	isZeroSize := b.targetData.TypeAllocSize(valueType) == 0
	var valueAlloca, valueAllocaSize llvm.Value
	if isZeroSize {
		valueAlloca = llvm.ConstNull(b.dataPtrType)
	} else {
		valueAlloca, valueAllocaSize = b.createTemporaryAlloca(valueType, "chan.value")
		b.CreateStore(chanValue, valueAlloca)
	}

	// Allocate buffer for the channel operation.
	channelOp := b.getLLVMRuntimeType("channelOp")
	channelOpAlloca, channelOpAllocaSize := b.createTemporaryAlloca(channelOp, "chan.op")

	// Do the send.
	b.createRuntimeCall("chanSend", []llvm.Value{ch, valueAlloca, channelOpAlloca}, "")

	// End the lifetime of the allocas.
	// This also works around a bug in CoroSplit, at least in LLVM 8:
	// https://bugs.llvm.org/show_bug.cgi?id=41742
	b.emitLifetimeEnd(channelOpAlloca, channelOpAllocaSize)
	if !isZeroSize {
		b.emitLifetimeEnd(valueAlloca, valueAllocaSize)
	}
}

// createChanRecv emits a pseudo chan receive operation. It is lowered to the
// actual channel receive operation during goroutine lowering.
func (b *builder) createChanRecv(unop *ssa.UnOp) llvm.Value {
	valueType := b.getLLVMType(unop.X.Type().Underlying().(*types.Chan).Elem())
	ch := b.getValue(unop.X, getPos(unop))

	// Allocate memory to receive into.
	isZeroSize := b.targetData.TypeAllocSize(valueType) == 0
	var valueAlloca, valueAllocaSize llvm.Value
	if isZeroSize {
		valueAlloca = llvm.ConstNull(b.dataPtrType)
	} else {
		valueAlloca, valueAllocaSize = b.createTemporaryAlloca(valueType, "chan.value")
	}

	// Allocate buffer for the channel operation.
	channelOp := b.getLLVMRuntimeType("channelOp")
	channelOpAlloca, channelOpAllocaSize := b.createTemporaryAlloca(channelOp, "chan.op")

	// Do the receive.
	commaOk := b.createRuntimeCall("chanRecv", []llvm.Value{ch, valueAlloca, channelOpAlloca}, "")
	var received llvm.Value
	if isZeroSize {
		received = llvm.ConstNull(valueType)
	} else {
		received = b.CreateLoad(valueType, valueAlloca, "chan.received")
		b.emitLifetimeEnd(valueAlloca, valueAllocaSize)
	}
	b.emitLifetimeEnd(channelOpAlloca, channelOpAllocaSize)

	if unop.CommaOk {
		tuple := llvm.Undef(b.ctx.StructType([]llvm.Type{valueType, b.ctx.Int1Type()}, false))
		tuple = b.CreateInsertValue(tuple, received, 0, "")
		tuple = b.CreateInsertValue(tuple, commaOk, 1, "")
		return tuple
	} else {
		return received
	}
}

// createChanClose closes the given channel.
func (b *builder) createChanClose(ch llvm.Value) {
	b.createRuntimeCall("chanClose", []llvm.Value{ch}, "")
}

// createSelect emits all IR necessary for a select statements. That's a
// non-trivial amount of code because select is very complex to implement.
func (b *builder) createSelect(expr *ssa.Select) llvm.Value {
	if len(expr.States) == 0 {
		// Shortcuts for some simple selects.
		llvmType := b.getLLVMType(expr.Type())
		if expr.Blocking {
			// Blocks forever:
			//     select {}
			b.createRuntimeCall("deadlock", nil, "")
			return llvm.Undef(llvmType)
		} else {
			// No-op:
			//     select {
			//     default:
			//     }
			retval := llvm.Undef(llvmType)
			retval = b.CreateInsertValue(retval, llvm.ConstInt(b.intType, 0xffffffffffffffff, true), 0, "")
			return retval // {-1, false}
		}
	}

	// This code create a (stack-allocated) slice containing all the select
	// cases and then calls runtime.chanSelect to perform the actual select
	// statement.
	// Simple selects (blocking and with just one case) are already transformed
	// into regular chan operations during SSA construction so we don't have to
	// optimize such small selects.

	// Go through all the cases. Create the selectStates slice and and
	// determine the receive buffer size and alignment.
	recvbufSize := uint64(0)
	recvbufAlign := 0
	var selectStates []llvm.Value
	chanSelectStateType := b.getLLVMRuntimeType("chanSelectState")
	for _, state := range expr.States {
		ch := b.getValue(state.Chan, state.Pos)
		selectState := llvm.ConstNull(chanSelectStateType)
		selectState = b.CreateInsertValue(selectState, ch, 0, "")
		switch state.Dir {
		case types.RecvOnly:
			// Make sure the receive buffer is big enough and has the correct alignment.
			llvmType := b.getLLVMType(state.Chan.Type().Underlying().(*types.Chan).Elem())
			if size := b.targetData.TypeAllocSize(llvmType); size > recvbufSize {
				recvbufSize = size
			}
			if align := b.targetData.ABITypeAlignment(llvmType); align > recvbufAlign {
				recvbufAlign = align
			}
		case types.SendOnly:
			// Store this value in an alloca and put a pointer to this alloca
			// in the send state.
			sendValue := b.getValue(state.Send, state.Pos)
			alloca := llvmutil.CreateEntryBlockAlloca(b.Builder, sendValue.Type(), "select.send.value")
			b.CreateStore(sendValue, alloca)
			selectState = b.CreateInsertValue(selectState, alloca, 1, "")
		default:
			panic("unreachable")
		}
		selectStates = append(selectStates, selectState)
	}

	// Create a receive buffer, where the received value will be stored.
	recvbuf := llvm.Undef(b.dataPtrType)
	if recvbufSize != 0 {
		allocaType := llvm.ArrayType(b.ctx.Int8Type(), int(recvbufSize))
		recvbufAlloca, _ := b.createTemporaryAlloca(allocaType, "select.recvbuf.alloca")
		recvbufAlloca.SetAlignment(recvbufAlign)
		recvbuf = b.CreateGEP(allocaType, recvbufAlloca, []llvm.Value{
			llvm.ConstInt(b.ctx.Int32Type(), 0, false),
			llvm.ConstInt(b.ctx.Int32Type(), 0, false),
		}, "select.recvbuf")
	}

	// Create the states slice (allocated on the stack).
	statesAllocaType := llvm.ArrayType(chanSelectStateType, len(selectStates))
	statesAlloca, statesSize := b.createTemporaryAlloca(statesAllocaType, "select.states.alloca")
	for i, state := range selectStates {
		// Set each slice element to the appropriate channel.
		gep := b.CreateGEP(statesAllocaType, statesAlloca, []llvm.Value{
			llvm.ConstInt(b.ctx.Int32Type(), 0, false),
			llvm.ConstInt(b.ctx.Int32Type(), uint64(i), false),
		}, "")
		b.CreateStore(state, gep)
	}
	statesPtr := b.CreateGEP(statesAllocaType, statesAlloca, []llvm.Value{
		llvm.ConstInt(b.ctx.Int32Type(), 0, false),
		llvm.ConstInt(b.ctx.Int32Type(), 0, false),
	}, "select.states")
	statesLen := llvm.ConstInt(b.uintptrType, uint64(len(selectStates)), false)

	// Do the select in the runtime.
	var results llvm.Value
	if expr.Blocking {
		// Stack-allocate operation structures.
		// If these were simply created as a slice, they would heap-allocate.
		opsAllocaType := llvm.ArrayType(b.getLLVMRuntimeType("channelOp"), len(selectStates))
		opsAlloca, opsSize := b.createTemporaryAlloca(opsAllocaType, "select.block.alloca")
		opsLen := llvm.ConstInt(b.uintptrType, uint64(len(selectStates)), false)
		opsPtr := b.CreateGEP(opsAllocaType, opsAlloca, []llvm.Value{
			llvm.ConstInt(b.ctx.Int32Type(), 0, false),
			llvm.ConstInt(b.ctx.Int32Type(), 0, false),
		}, "select.block")

		results = b.createRuntimeCall("chanSelect", []llvm.Value{
			recvbuf,
			statesPtr, statesLen, statesLen, // []chanSelectState
			opsPtr, opsLen, opsLen, // []channelOp
		}, "select.result")

		// Terminate the lifetime of the operation structures.
		b.emitLifetimeEnd(opsAlloca, opsSize)
	} else {
		opsPtr := llvm.ConstNull(b.dataPtrType)
		opsLen := llvm.ConstInt(b.uintptrType, 0, false)
		results = b.createRuntimeCall("chanSelect", []llvm.Value{
			recvbuf,
			statesPtr, statesLen, statesLen, // []chanSelectState
			opsPtr, opsLen, opsLen, // []channelOp (nil slice)
		}, "select.result")
	}

	// Terminate the lifetime of the states alloca.
	b.emitLifetimeEnd(statesAlloca, statesSize)

	// The result value does not include all the possible received values,
	// because we can't load them in advance. Instead, the *ssa.Extract
	// instruction will treat a *ssa.Select specially and load it there inline.
	// Store the receive alloca in a sidetable until we hit this extract
	// instruction.
	if b.selectRecvBuf == nil {
		b.selectRecvBuf = make(map[*ssa.Select]llvm.Value)
	}
	b.selectRecvBuf[expr] = recvbuf

	return results
}

// getChanSelectResult returns the special values from a *ssa.Extract expression
// when extracting a value from a select statement (*ssa.Select). Because
// *ssa.Select cannot load all values in advance, it does this later in the
// *ssa.Extract expression.
func (b *builder) getChanSelectResult(expr *ssa.Extract) llvm.Value {
	if expr.Index == 0 {
		// index
		value := b.getValue(expr.Tuple, getPos(expr))
		index := b.CreateExtractValue(value, expr.Index, "")
		if index.Type().IntTypeWidth() < b.intType.IntTypeWidth() {
			index = b.CreateSExt(index, b.intType, "")
		}
		return index
	} else if expr.Index == 1 {
		// comma-ok
		value := b.getValue(expr.Tuple, getPos(expr))
		return b.CreateExtractValue(value, expr.Index, "")
	} else {
		// Select statements are (index, ok, ...) where ... is a number of
		// received values, depending on how many receive statements there
		// are. They are all combined into one alloca (because only one
		// receive can proceed at a time) so we'll get that alloca, bitcast
		// it to the correct type, and dereference it.
		recvbuf := b.selectRecvBuf[expr.Tuple.(*ssa.Select)]
		typ := b.getLLVMType(expr.Type())
		return b.CreateLoad(typ, recvbuf, "")
	}
}
