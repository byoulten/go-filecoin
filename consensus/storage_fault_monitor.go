package consensus

import (
	"context"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

const (
	// AfterProvingPeriod indicates the miner did not submitPoSt within the proving period
	AfterProvingPeriod = 51
	// AfterGenerationAttackThreshold indicates the miner did not submitPoSt within the
	// Generation Attack Threshold
	AfterGenerationAttackThreshold = 52
	// MissingProof indicates there was no submitPoSt message from miner during the proving
	// period, but one was expected
	MissingProof = 53
)

// TSIter is an iterator over a TipSet
type TSIter interface {
	Complete() bool
	Next() error
	Value() types.TipSet
}

// MonitorPorcelain is an interface for the functionality StorageFaultMonitor needs
type MonitorPorcelain interface {
	ActorLs(context.Context) (<-chan state.GetAllActorsResult, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
}

// StorageFault holds a record of a miner storage fault
type StorageFault struct {
	Code      uint8
	MinerAddr address.Address
	AtHeight  uint64
}

// StorageFaultMonitor checks each new tipset for storage faults, a.k.a. market faults.
// Storage faults are distinct from consensus faults.
// See https://github.com/filecoin-project/specs/blob/master/faults.md
type StorageFaultMonitor struct {
	ctx           context.Context
	log           logging.EventLogger
	minerHasPower func(ctx context.Context, minerAddr address.Address, ts types.TipSet) (bool, error)
	newTs         types.TipSet
	porc          MonitorPorcelain
}

// NewStorageFaultMonitor creates a new StorageFaultMonitor with the provided porcelain and function
// to get miner power
func NewStorageFaultMonitor(
	porcelain MonitorPorcelain,
	hasPowerFunc func(ctx context.Context, minerAddr address.Address, ts types.TipSet) (bool, error),
) *StorageFaultMonitor {
	return &StorageFaultMonitor{
		porc:          porcelain,
		minerHasPower: hasPowerFunc,
		log:           logging.Logger("StorageFaultMonitor"),
	}
}

// HandleNewTipSet receives an iterator over the current chain, and a new tipset
// and looks for missing, expected submitPoSts
// Miners without power and those that posted proofs to newTs are skipped
func (sfm *StorageFaultMonitor) HandleNewTipSet(ctx context.Context, iter TSIter, newTs types.TipSet) (*[]StorageFault, error) {

	sfm.newTs = newTs
	sfm.ctx = ctx

	var emptyFaults []StorageFault

	// iterate over blocks in the new tipset and detect faults
	// Maybe hash all the miner addresses with submitPoSts first & then go down the chain once
	head := iter.Value()
	_, err := head.Height()
	if err != nil {
		return &emptyFaults, err
	}
	var minersSeen []address.Address

	// collect all miners submitting PoSts this tipset
	for i := 0; i < head.Len(); i++ {
		blk := head.At(i)
		for _, msg := range blk.Messages {
			m := msg.Method
			switch m {
			case "submitPost":
				// Miners sending sumbitPost messages are self-reporting & self-penalizing
				minersSeen = append(minersSeen, msg.From)
			default:
				continue
			}
		}
	}
	minersToCheck, err := sfm.unseenMinersWithPower(&minersSeen)
	if err != nil {
		return &emptyFaults, err
	}

	// collect miner actor proving periods & power
	stats, err := sfm.minerStats(minersToCheck)
	if err != nil {
		return &emptyFaults, err
	}

	faults, err := sfm.collectMinerFaults(stats)

	// traverse the chain back
	return faults, nil
}

// unseenMinersWithPower iterates over actors returned from ActorLs and returns a list of
// miner addresses for miners with power only, except the ones provided by minersSeen
func (sfm *StorageFaultMonitor) unseenMinersWithPower(minersSeen *[]address.Address) ([]address.Address, error) {
	var miners []address.Address

	// collect miner actors in actorLS
	actorsCh, err := sfm.porc.ActorLs(sfm.ctx)
	if err != nil {
		return miners, err
	}
	for actor := range actorsCh {
		if actor.Error != nil {
			return miners, actor.Error
		}
		if !actor.Actor.Code.Equals(types.MinerActorCodeCid) {
			continue
		}

		miner, err := address.NewFromString(actor.Address)
		if err != nil {
			return miners, err
		}

		// subtract minersSeen
		if indexOfAddress(minersSeen, miner) >= 0 {
			continue
		}

		hasPower, err := sfm.minerHasPower(sfm.ctx, miner, sfm.newTs)
		if err != nil {
			return miners, err
		}

		if hasPower {
			miners = append(miners, miner)
		}
	}
	return miners, nil
}

type minerStats struct {
	MinerAddr          address.Address
	ProvingPeriodStart *types.BlockHeight
	ProvingPeriodEnd   *types.BlockHeight
}

// minerStats collects proving period starts and ends for each miner
// Note this causes a message post for each miner
func (sfm *StorageFaultMonitor) minerStats(minerAddrs []address.Address) (*[]minerStats, error) {

	var result []minerStats
	return &result, nil
}

func (sfm *StorageFaultMonitor) collectMinerFaults(stats *[]minerStats) (*[]StorageFault, error) {
	var result []StorageFault

	height, err := sfm.newTs.Height()
	if err != nil {
		return &result, err
	}

	for _, ms := range *stats {
		sf := StorageFault{
			MinerAddr: ms.MinerAddr,
			Code:      AfterProvingPeriod,
			AtHeight:  height,
		}
		result = append(result, sf)
	}
	return &result, nil
}

// MinerLastSeen returns the block height at which miner last sent a `submitPost` message, or
// nil if it was not seen within lookBackLimit blocks ago, not counting the head.
// Is it useful to cache the block height at which the miner was last seen? If not this can just return
// a bool + error
func MinerLastSeen(miner address.Address, iter TSIter, lookBackLimit *types.BlockHeight) (*types.BlockHeight, error) {
	// iterate in the rest of the chain and check head against rest of chain for faults
	var err error

	for i := uint64(0); !iter.Complete() && lookBackLimit.GreaterThan(types.NewBlockHeight(i)); i++ {
		if err = iter.Next(); err != nil {
			return nil, err
		}

		ts := iter.Value()
		for j := 0; j < ts.Len(); j++ {
			blk := ts.At(j)
			if msg := poStMessageFrom(miner, blk.Messages); msg != nil {
				h, err := ts.Height()
				if err != nil {
					return nil, err
				}
				return types.NewBlockHeight(h), nil
			}
		}
	}
	return nil, nil
}

func poStMessageFrom(miner address.Address, msgs []*types.SignedMessage) (msg *types.SignedMessage) {
	for _, msg = range msgs {
		meth := msg.Method
		from := msg.From.String()
		if from == miner.String() && meth == "submitPost" {
			return msg
		}
	}
	return nil
}

// indexOfAddress searches for the given address in the list.
// Returns -1 if the address is not found
func indexOfAddress(addrList *[]address.Address, addr address.Address) int64 {
	for i, a := range *addrList {
		if a.String() == addr.String() {
			return int64(i)
		}
	}
	return -1
}
