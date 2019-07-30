package tests

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	logging "github.com/ipfs/go-log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestSlashing(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("works", func(t *testing.T) {
		logging.SetDebugLogging()

		// Give the deal time to complete
		ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{
			InitOpts:   []fast.ProcessInitOption{fast.POAutoSealIntervalSeconds(1)},
			DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(50 * time.Millisecond)},
		})
		defer func() {
			require.NoError(t, env.Teardown(ctx))
		}()
		clientDaemon := env.GenesisMiner
		require.NoError(t, clientDaemon.MiningStart(ctx))
		defer func() {
			require.NoError(t, clientDaemon.MiningStop(ctx))
		}()

		minerDaemon := env.RequireNewNodeWithFunds(1111)

		duration := uint64(5)
		askID := requireMinerCreateWithAsk(ctx, t, minerDaemon)
		dealResponse := requireMinerClientMakeADeal(ctx, t, minerDaemon, clientDaemon, askID, duration)

		// atLeastStartH is either the start height of the deal or a height after the deal has started.
		atLeastStartH, err := series.GetHeadBlockHeight(ctx, clientDaemon)
		require.NoError(t, err)

		// Wait until deal is accepted
		err = series.WaitForDealState(ctx, clientDaemon, dealResponse, storagedeal.Staged)
		require.NoError(t, err)

		require.NoError(t, series.WaitForBlockHeight(ctx, minerDaemon, atLeastStartH.Add(types.NewBlockHeight(duration+1))))

		// Wait until proving period is over.
		waitingPeriod := atLeastStartH.Add(types.NewBlockHeight(1001))
		require.NoError(t, series.WaitForBlockHeight(ctx, minerDaemon, waitingPeriod))

		// assert that miner has power now
		assertHasPower(ctx, t, minerDaemon, 1024)

		// miner makes another deal
		duration = uint64(2001)
		dealResponse = requireMinerClientMakeADeal(ctx, t, minerDaemon, clientDaemon, askID, duration)
		err = series.WaitForDealState(ctx, clientDaemon, dealResponse, storagedeal.Staged)

		require.NoError(t, err)

		// miner is offline
		require.NoError(t, minerDaemon.StopDaemon(ctx))

		atLeastStartH, err = series.GetHeadBlockHeight(ctx, clientDaemon)
		require.NoError(t, err)

		waitingPeriod = atLeastStartH.Add(types.NewBlockHeight(1001))

		require.NoError(t, series.WaitForBlockHeight(ctx, clientDaemon, waitingPeriod))

		_, err = minerDaemon.StartDaemon(context.Background(), true)
		require.NoError(t, err)

		// assert miner has been slashed
		assertHasPower(ctx, t, minerDaemon, 0)
	})

	// start genesis node mining
	// set up another miner with commits
	// verify normal operation of storage fault monitor when there is a new tipset
	//  (it doesn't crash?)

	// 0. make miner submit proof on time
	//    verify normal operation of storage fault monitor
	//    verify miner is not slashed

	// 1. make miner be late by not submitting proof
	//    verify the miner is slashed

	// 2. make miner be late but submits late proof
	//    verify miner is not slashed twice

}

func requireMinerCreateWithAsk(ctx context.Context, t *testing.T, d *fast.Filecoin) uint64 {
	collateral := big.NewInt(int64(100))
	askPrice := big.NewFloat(0.5)
	expiry := big.NewInt(int64(10000))
	ask, err := series.CreateStorageMinerWithAsk(ctx, d, collateral, askPrice, expiry)
	require.NoError(t, err)
	return ask.ID
}

func requireMinerClientMakeADeal(ctx context.Context, t *testing.T, minerDaemon, clientDaemon *fast.Filecoin, askID uint64, duration uint64) *storagedeal.Response {
	f := files.NewBytesFile([]byte("HODLHODLHODL"))
	dataCid, err := clientDaemon.ClientImport(ctx, f)
	require.NoError(t, err)

	minerAddress := requireGetMinerAddress(ctx, t, minerDaemon)

	dealResponse, err := clientDaemon.ClientProposeStorageDeal(ctx, dataCid, minerAddress, askID, duration, fast.AOAllowDuplicates(true))

	require.NoError(t, err)
	return dealResponse
}

func requireGetMinerAddress(ctx context.Context, t *testing.T, daemon *fast.Filecoin) address.Address {
	var minerAddress address.Address
	err := daemon.ConfigGet(ctx, "mining.minerAddress", &minerAddress)
	require.NoError(t, err)
	return minerAddress
}

func assertHasPower(ctx context.Context, t *testing.T, d *fast.Filecoin, expPower int64) {
	actualPower, _, err := d.MinerPower(ctx, requireGetMinerAddress(ctx, t, d))
	require.NoError(t, err)
	assert.Equal(t, expPower, actualPower.Int64())
}
