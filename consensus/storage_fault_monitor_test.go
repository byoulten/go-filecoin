package consensus_test

import (
	"context"
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	. "github.com/filecoin-project/go-filecoin/consensus"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestHandleNewTipSet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	t.Run("When no miners, empty LateMiners", func(t *testing.T) {
		//st, vms := th.RequireCreateStorages(ctx, t)

		height := types.NewBlockHeight(1)
		data, err := cbor.DumpObject(&map[string]uint64{})
		require.NoError(t, err)

		fm := NewStorageFaultMonitor(NewDefaultTestMinerPorcelain(false, false, makeQueryer([][]byte{data})), address.TestAddress)
		err = fm.HandleNewTipSet(ctx, height)
		require.NoError(t, err)
	})

}

func makeQueryer(returnData [][]byte) msgQueryer {
	return func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
		return returnData, nil
	}
}

// 5 actors
// 1. 3 with power and no submitPoSts
func TestHandleNewTipSet_NoMinersSubmitPoSt(t *testing.T) {
}

// 2. 3 with power and all submitPoSts
// 3. 5 with power and 2 submitPoSts, remaining 3 posted within proving periods
// 4. 5 with power and 2 submitPoSts, 1 posted within proving period, 1 posted before GAT, 1 missing completely

func TestStorageFaultMonitor_HandleNewTipSet_LateSubmission(t *testing.T) {
}

func RequireTipset(t *testing.T, blocks ...*types.Block) types.TipSet {
	set, err := types.NewTipSet(blocks...)
	require.NoError(t, err)
	return set
}

type msgQueryer func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)

type TestMinerPorcelain struct {
	actorFail   bool
	actorChFail bool
	Queryer     msgQueryer
}

func NewDefaultTestMinerPorcelain(actorFail, actorChFail bool, queryer msgQueryer) *TestMinerPorcelain {
	tmp := TestMinerPorcelain{
		actorChFail: actorChFail,
		actorFail:   actorFail,
		Queryer:     queryer,
	}
	return &tmp
}

func (tmp *TestMinerPorcelain) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	return tmp.Queryer(ctx, optFrom, to, method, params)
}
