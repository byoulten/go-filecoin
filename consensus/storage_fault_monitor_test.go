package consensus_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	. "github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestHandleNewTipSet_AllMinersSubmitPoSt(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	mm, chainer, root := makeMessageMakerChainerRoot(t, 2)

	beyonce := mm.Addresses()[0]
	davante := mm.Addresses()[1]

	q := core.NewMessageQueue()
	msgs := []*types.SignedMessage{
		RequireEnqueue(ctx, t, q, mm.NewSubmiPoStMsg(beyonce, 1), 100),
		RequireEnqueue(ctx, t, q, mm.NewSubmiPoStMsg(davante, 2), 101),
	}

	newBlk := chainer.NewBlockWithMessages(1, msgs, root)
	t1 := RequireTipset(t, newBlk)
	iter := chain.IterAncestors(ctx, chainer, t1)

	fm := NewStorageFaultMonitor(&TestMinerPorcelain{}, alwaysHasPower)
	faults, err := fm.HandleNewTipSet(ctx, iter, t1)
	assert.NoError(t, err)
	assert.Empty(t, faults)
}

func TestHandleNewTipSet_NoMinersHavePower(t *testing.T) {
	ctx := context.Background()
	mm, chainer, root := makeMessageMakerChainerRoot(t, 2)

	beyonce := mm.Addresses()[0]
	davante := mm.Addresses()[1]

	q := core.NewMessageQueue()
	msgs := []*types.SignedMessage{
		RequireEnqueue(ctx, t, q, mm.NewSignedMessage(beyonce, 1), 100),
		RequireEnqueue(ctx, t, q, mm.NewSignedMessage(davante, 2), 101),
	}
	newBlk := chainer.NewBlockWithMessages(1, msgs, root)
	t1 := RequireTipset(t, newBlk)
	iter := chain.IterAncestors(ctx, chainer, t1)

	fm := NewStorageFaultMonitor(&TestMinerPorcelain{}, neverHasPower)
	faults, err := fm.HandleNewTipSet(ctx, iter, t1)
	assert.NoError(t, err)
	assert.Empty(t, faults)

}

func makeMessageMakerChainerRoot(t *testing.T, howMany int) (*types.MessageMaker, *th.FakeChainProvider, *types.Block) {
	keys := types.MustGenerateKeyInfo(howMany, 42)
	mm := types.NewMessageMaker(t, keys)

	chainer := th.NewFakeChainProvider()
	root := chainer.NewBlock(0)

	return mm, chainer, root
}

// 5 actors
// 1. 3 with power and no submitPoSts
func TestHandleNewTipSet_NoMinersSubmitPoSt(t *testing.T) {
	ctx := context.Background()

	mm, chainer, root := makeMessageMakerChainerRoot(t, 5)

	ape := mm.Addresses()[0]
	bat := mm.Addresses()[1]
	cat := mm.Addresses()[2]
	dog := mm.Addresses()[3]
	elf := mm.Addresses()[4]

	q := core.NewMessageQueue()
	msgs := []*types.SignedMessage{
		RequireEnqueue(ctx, t, q, mm.NewSignedMessage(dog, 1), 100),
		RequireEnqueue(ctx, t, q, mm.NewSignedMessage(elf, 2), 101),
	}

	newBlk := chainer.NewBlockWithMessages(1, msgs, root)
	t1 := RequireTipset(t, newBlk)
	iter := chain.IterAncestors(ctx, chainer, t1)

	hasPower := func(ctx context.Context, minerAddr address.Address, ts types.TipSet) (bool, error) {
		mastr := minerAddr.String()
		if mastr == ape.String() || mastr == bat.String() || mastr == cat.String() {
			return true, nil
		}
		return false, nil
	}

	fm := NewStorageFaultMonitor(&TestMinerPorcelain{}, hasPower)
	faults, err := fm.HandleNewTipSet(ctx, iter, t1)
	require.NoError(t, err)
	require.NotNil(t, faults)
	assert.NoError(t, err)
	assert.Len(t, *faults, 3)

}

// 2. 3 with power and all submitPoSts
// 3. 5 with power and 2 submitPoSts, remaining 3 posted within proving periods
// 4. 5 with power and 2 submitPoSts, 1 posted within proving period, 1 posted before GAT, 1 missing completely

func TestStorageFaultMonitor_HandleNewTipSet_LateSubmission(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	mm, chainer, root := makeMessageMakerChainerRoot(t, 2)

	xavier := mm.Addresses()[0]
	yoli := mm.Addresses()[1]

	q := core.NewMessageQueue()
	msgs := []*types.SignedMessage{
		RequireEnqueue(ctx, t, q, mm.NewSubmiPoStMsg(xavier, 1), 100),
		RequireEnqueue(ctx, t, q, mm.NewSignedMessage(yoli, 2), 101),
	}

	q1 := chainer.NewBlockWithMessages(3, msgs, root)
	q2 := chainer.NewBlockWithMessages(4, []*types.SignedMessage{}, q1, root)
	q3 := chainer.NewBlockWithMessages(5, []*types.SignedMessage{}, q2, q1, root)
	q4 := chainer.NewBlockWithMessages(6, []*types.SignedMessage{}, q3, q2, q1, root)

	t1 := RequireTipset(t, q4)
	iter := chain.IterAncestors(ctx, chainer, t1)

	fm := NewStorageFaultMonitor(&TestMinerPorcelain{}, alwaysHasPower)
	faults, err := fm.HandleNewTipSet(ctx, iter, t1)
	assert.NoError(t, err)
	assert.Empty(t, faults)
}

func TestMinerLastSeen(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	keys := types.MustGenerateKeyInfo(2, 42)
	mm := types.NewMessageMaker(t, keys)

	beyonce := mm.Addresses()[0]
	davante := mm.Addresses()[1]

	chainer := th.NewFakeChainProvider()
	root := chainer.NewBlock(0)

	// block height = 1
	q1 := core.NewMessageQueue()
	q1msgs := []*types.SignedMessage{
		RequireEnqueue(ctx, t, q1, mm.NewSubmiPoStMsg(beyonce, 1), 100),
		RequireEnqueue(ctx, t, q1, mm.NewSignedMessage(davante, 2), 101),
	}
	q1blk := chainer.NewBlockWithMessages(1, q1msgs, root)

	// block height = 2
	q2 := core.NewMessageQueue()
	q2Msgs := []*types.SignedMessage{
		RequireEnqueue(ctx, t, q2, mm.NewSubmiPoStMsg(davante, 3), 102),
		RequireEnqueue(ctx, t, q2, mm.NewSignedMessage(davante, 4), 103),
	}
	q2blk := chainer.NewBlockWithMessages(2, q2Msgs, q1blk)

	t2 := RequireTipset(t, q2blk)

	t.Run("returns nil when miner not seen submitting post on chain at all", func(t *testing.T) {
		iter := chain.IterAncestors(ctx, chainer, t2)
		assertNeverSeen(t, iter, address.TestAddress2, 1)
	})

	t.Run("returns block height 1 when sees submit post msg", func(t *testing.T) {
		// beyonce node submitted a post
		iter := chain.IterAncestors(ctx, chainer, t2)
		assertLastSeenAt(t, iter, beyonce, 1, 1)
	})
	t.Run("returns nil when submitPost msg not found within limit", func(t *testing.T) {
		// davante node did not submit a post within lookup limit of 2
		iter := chain.IterAncestors(ctx, chainer, t2)
		assertNeverSeen(t, iter, davante, 2)
	})

	t.Run("finds the submitPost for miner after added block", func(t *testing.T) {
		// Simulate another block coming in with davante node's PoSt msg
		// block height = 3
		q3 := core.NewMessageQueue()
		q3Msgs := []*types.SignedMessage{
			RequireEnqueue(ctx, t, q3, mm.NewSubmiPoStMsg(davante, 4), 104),
		}
		q3blk := chainer.NewBlockWithMessages(2, q3Msgs, q2blk)

		t3 := RequireTipset(t, q3blk)
		iter := chain.IterAncestors(ctx, chainer, t3)

		assertLastSeenAt(t, iter, davante, 3, 2)

		// reset head, test that the iterator really stops when it should.
		iter = chain.IterAncestors(ctx, chainer, t3)

		assertNeverSeen(t, iter, beyonce, 1)

		// reset head, test that the iterator really stops when it should.
		iter = chain.IterAncestors(ctx, chainer, t3)
		assertLastSeenAt(t, iter, beyonce, 3, 1)

	})
}

func RequireTipset(t *testing.T, blocks ...*types.Block) types.TipSet {
	set, err := types.NewTipSet(blocks...)
	require.NoError(t, err)
	return set
}

func RequireEnqueue(ctx context.Context, t *testing.T, q *core.MessageQueue, msg *types.SignedMessage, stamp uint64) *types.SignedMessage {
	err := q.Enqueue(ctx, msg, stamp)
	require.NoError(t, err)
	return msg
}

type TestMinerPorcelain struct {
	actorFail           bool
	actorChFail         bool
	ActorLister         func(ctx context.Context) (<-chan state.GetAllActorsResult, error)
	ProvingPeriodGetter func(context.Context, address.Address) (*types.BlockHeight, *types.BlockHeight, error)
	GATGetter           func(ctx context.Context, miner address.Address) (*types.BlockHeight, error)
	MessageQueryer      func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
}

func NewDefaultTestMinerPorcelain(actors []address.Address) *TestMinerPorcelain {
	tmp := TestMinerPorcelain{}
	// replace by querying storage market for storage actors?
	tmp.ActorLister = tmp.makeActorLs(actors)
	tmp.MessageQueryer = tmp.MessageQuery
	return &tmp
}

func (tmp *TestMinerPorcelain) makeActorLs(actorAddrs []address.Address) func(ctx context.Context) (<-chan state.GetAllActorsResult, error) {
	return func(ctx context.Context) (<-chan state.GetAllActorsResult, error) {
		out := make(chan state.GetAllActorsResult)

		if tmp.actorFail {
			return nil, errors.New("ACTOR FAILURE")
		}

		go func() {
			defer close(out)
			for i, addr := range actorAddrs {
				if tmp.actorChFail {
					out <- state.GetAllActorsResult{
						Error: errors.New("ACTOR CHANNEL FAILURE"),
					}
				} else {
					act := actor.Actor{Code: types.MinerActorCodeCid}
					out <- state.GetAllActorsResult{
						Address: addr.String(),
						Actor:   &act,
					}
				}
			}
		}()

		return out, nil

	}
}

func (tmp *TestMinerPorcelain) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	return [][]byte{}, nil
}

func assertLastSeenAt(t *testing.T, iter TSIter, miner address.Address, limit, expectedBh uint64) {
	bh, err := MinerLastSeen(miner, iter, types.NewBlockHeight(limit))
	require.NoError(t, err)
	require.NotNil(t, bh)
	assert.True(t, bh.Equal(types.NewBlockHeight(expectedBh)))
}

func assertNeverSeen(t *testing.T, iter TSIter, miner address.Address, limit uint64) {
	bh, err := MinerLastSeen(miner, iter, types.NewBlockHeight(limit))
	require.NoError(t, err)
	assert.Nil(t, bh)
}

func alwaysHasPower(context.Context, address.Address, types.TipSet) (bool, error) {
	return true, nil
}

func neverHasPower(context.Context, address.Address, types.TipSet) (bool, error) {
	return false, nil
}
