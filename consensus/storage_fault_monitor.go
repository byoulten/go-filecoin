package consensus

import (
	"context"

	"github.com/ipfs/go-cid"

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
	MinerGetProvingPeriod(context.Context, address.Address) (*types.BlockHeight, *types.BlockHeight, error)
	MinerGetGenerationAttackThreshold(context.Context, address.Address) (*types.BlockHeight, error)
}

// StorageFault holds a record of a miner storage fault
type StorageFault struct {
	Code     uint8
	Miner    address.Address
	BlockCID cid.Cid
	// LastSeenBlockHeight?
}

// StorageFaultMonitor checks each new tipset for storage faults, a.k.a. market faults.
// Storage faults are distinct from consensus faults.
// See https://github.com/filecoin-project/specs/blob/master/faults.md
type StorageFaultMonitor struct {
	porc          MonitorPorcelain
	log           logging.EventLogger
	minerHasPower func(ctx context.Context, minerAddr address.Address, ts types.TipSet) (bool, error)
	newTs         types.TipSet
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
// and checks the new tipset for storage faults, iterating over iter
func (sfm *StorageFaultMonitor) HandleNewTipSet(ctx context.Context, iter TSIter, newTs types.TipSet) ([]*StorageFault, error) {

	sfm.newTs = newTs

	var emptyFaults []*StorageFault

	// iterate over blocks in the new tipset and detect faults
	// Maybe hash all the miner addresses with submitPoSts first & then go down the chain once
	head := iter.Value()
	_, err := head.Height()
	if err != nil {
		return emptyFaults, err
	}
	var minersSeen []address.Address
	faults := emptyFaults

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
	// collect miner actors in actorLS
	// subtract minersSeen
	// collect miner actor proving periods & power
	// traverse the chain back
	return faults, nil
}

// minerActorsWithPower iterates over actors returned from ActorLs and returns a list of
// miner addresses for miners with power onl
func (sfm *StorageFaultMonitor) minerActorsWithPower(ctx context.Context) ([]address.Address, error) {
	var miners []address.Address

	actorsCh, err := sfm.porc.ActorLs(ctx)
	if err != nil {
		return miners, err
	}
	for a := range actorsCh {
		if a.Error != nil {
			return miners, a.Error
		}
		if !a.Actor.Code.Equals(types.MinerActorCodeCid) {
			continue
		}

		miner, err := address.NewFromString(a.Address)
		if err != nil {
			return miners, err
		}
		hasPower, err := sfm.minerHasPower(ctx, miner, sfm.newTs)
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
	ProvingPeriodStart types.BlockHeight
	ProvingPeriodEnd   types.BlockHeight
}

// minerStats collects proving period starts and ends for each miner
// Note this causes a message post for each miner
func (sfm *StorageFaultMonitor) minerStats(ctx context.Context, minerAddrs []address.Address) (*map[address.Address]minerStats, error) {

	result := make(map[address.Address]minerStats)

	// populate with minerAddrs with power, plus their proving period values
	for _, m := range minerAddrs {
		result[m] = minerStats{}
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
