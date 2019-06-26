package consensus_test

import (
	"context"
	"testing"

	"github.com/ipfs/go-log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestStorageFaultMonitor_HandleNewTipSet(t *testing.T) {
	tf.UnitTest(t)
	log.SetDebugLogging()

	ctx := context.Background()
	keys := types.MustGenerateKeyInfo(2, 42)
	mm := types.NewMessageMaker(t, keys)

	beyonce := mm.Addresses()[0]
	davante := mm.Addresses()[1]

	chainer := testhelpers.NewFakeChainProvider()
	_ = chainer.NewBlock(0)

	q := core.NewMessageQueue()
	msgs := []*types.SignedMessage{
		requireEnqueue(ctx, t, q, mm.NewSubmiPoStMsg(beyonce, 1), 100),
		requireEnqueue(ctx, t, q, mm.NewSignedMessage(davante, 2), 101),
	}

	newBlk := chainer.NewBlockWithMessages(1, msgs)
	t1 := requireTipset(t, newBlk)
	iter := chain.IterAncestors(ctx, chainer, t1)

	fm := consensus.NewStorageFaultMonitor(&testMinerPorcelain{}, beyonce)
	err := fm.HandleNewTipSet(ctx, iter, t1)
	assert.NoError(t, err)
}

type testMinerPorcelain struct{}

func (tmp *testMinerPorcelain) MinerGetProvingPeriod(context.Context, address.Address) (*types.BlockHeight, *types.BlockHeight, error) {
	return types.NewBlockHeight(1), types.NewBlockHeight(2), nil
}

func requireTipset(t *testing.T, blocks ...*types.Block) types.TipSet {
	set, err := types.NewTipSet(blocks...)
	require.NoError(t, err)
	return set
}

func requireEnqueue(ctx context.Context, t *testing.T, q *core.MessageQueue, msg *types.SignedMessage, stamp uint64) *types.SignedMessage {
	err := q.Enqueue(ctx, msg, stamp)
	require.NoError(t, err)
	return msg
}
