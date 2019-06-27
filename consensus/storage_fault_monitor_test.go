package consensus_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	. "github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestStorageFaultMonitor_HandleNewTipSet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	keys := types.MustGenerateKeyInfo(2, 42)
	mm := types.NewMessageMaker(t, keys)

	beyonce := mm.Addresses()[0]
	davante := mm.Addresses()[1]

	chainer := th.NewFakeChainProvider()
	_ = chainer.NewBlock(0)

	q := core.NewMessageQueue()
	msgs := []*types.SignedMessage{
		RequireEnqueue(ctx, t, q, mm.NewSubmiPoStMsg(beyonce, 1), 100),
		RequireEnqueue(ctx, t, q, mm.NewSignedMessage(davante, 2), 101),
	}

	newBlk := chainer.NewBlockWithMessages(1, msgs)
	t1 := RequireTipset(t, newBlk)
	iter := chain.IterAncestors(ctx, chainer, t1)

	fm := NewStorageFaultMonitor(&TestMinerPorcelain{}, beyonce)
	err := fm.HandleNewTipSet(ctx, iter, t1)
	assert.NoError(t, err)
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

type TestMinerPorcelain struct{}

func (tmp *TestMinerPorcelain) MinerGetProvingPeriod(context.Context, address.Address) (*types.BlockHeight, *types.BlockHeight, error) {
	return types.NewBlockHeight(1), types.NewBlockHeight(2), nil
}
func (tmp *TestMinerPorcelain) MinerGetGenerationAttackThreshold(ctx context.Context, miner address.Address) *types.BlockHeight {
	return types.NewBlockHeight(100)
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
