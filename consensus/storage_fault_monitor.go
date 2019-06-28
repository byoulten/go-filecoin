package consensus

import (
	"context"
	"github.com/ipfs/go-cid"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// TSIter is an iterator over a TipSet
type TSIter interface {
	Complete() bool
	Next() error
	Value() types.TipSet
}

// MonitorPorcelain is an interface for the functionality StorageFaultMonitor needs
type MonitorPorcelain interface {
	MinerGetProvingPeriod(context.Context, address.Address) (*types.BlockHeight, *types.BlockHeight, error)
	MinerGetGenerationAttackThreshold(context.Context, address.Address) (*types.BlockHeight, error)
}

const (
	// LateSubmission indicates the miner did not submitPoSt within the proving period
	LateSubmission = 51
	// AfterGenerationAttackThreshold indicates the miner did not submitPoSt within the
	// Generation Attack Threshold
	AfterGenerationAttackThreshold = 52
	// EmptyProofs indicates the proofs array is empty for the submitPoSt message
	EmptyProofs = 53
)

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
	minerAddr address.Address
	porc      MonitorPorcelain
	pdStart   *types.BlockHeight
	pdEnd     *types.BlockHeight
	gat       *types.BlockHeight
	log       logging.EventLogger
}

// NewStorageFaultMonitor creates a new StorageFaultMonitor with the provided porcelain
func NewStorageFaultMonitor(porcelain MonitorPorcelain, minerAddr address.Address) *StorageFaultMonitor {
	return &StorageFaultMonitor{
		minerAddr: minerAddr,
		porc:      porcelain,
		log:       logging.Logger("StorageFaultMonitor"),
	}
}

// HandleNewTipSet receives an iterator over the current chain, and a new tipset
// and checks the new tipset for storage faults, iterating over iter
func (sfm *StorageFaultMonitor) HandleNewTipSet(ctx context.Context, iter TSIter, newTs types.TipSet) ([]*StorageFault, error) {
	var emptyFaults []*StorageFault

	// iterate over blocks in the new tipset and detect faults
	// Maybe hash all the miner addresses with submitPoSts first & then go down the chain once
	head := iter.Value()
	bh, err := head.Height()
	if err != nil {
		return emptyFaults, err
	}

	sfm.pdStart, sfm.pdEnd, err = sfm.porc.MinerGetProvingPeriod(ctx, sfm.minerAddr)
	if err != nil {
		return emptyFaults, err
	}

	sfm.gat, err = sfm.porc.MinerGetGenerationAttackThreshold(ctx, sfm.minerAddr)
	if err != nil {
		return emptyFaults, err
	}
	faults := emptyFaults

	for i := 0; i < head.Len(); i++ {
		blk := head.At(i)
		for _, msg := range blk.Messages {
			m := msg.Method
			miner := msg.From
			switch m {
			case "submitPost":

				if emptyProofs(msg) {
					fault := StorageFault{
						BlockCID: blk.Cid(),
						Code:     EmptyProofs,
						Miner:    miner,
					}

					faults = append(faults, &fault)
					sfm.log.Debug("submitPost message missing proof at blockheight %d for miner %s", bh, miner.String())
					continue
				}

				curHeight := types.NewBlockHeight(bh)

				//check provided proof(s) for late submission
				lateSubmissionLimit := curHeight.Sub(sfm.pdStart)
				lastSeen, err := MinerLastSeen(miner, iter, lateSubmissionLimit)
				if err != nil {
					return faults, err
				}
				if lastSeen == nil {
					fault := StorageFault{
						BlockCID: blk.Cid(),
						Miner:    miner,
					}

					// check for submission before generation attack threshold
					// this continues the iterator where we left off and looks an additional
					// generation-attack-threshold block height.
					lastSeen, err = MinerLastSeen(miner, iter, sfm.gat)
					if err != nil {
						return faults, err
					}

					if lastSeen == nil {
						sfm.log.Debug("submitPoSt not seen within generation attack threshold")
						fault.Code = AfterGenerationAttackThreshold
					} else {
						sfm.log.Debug("submitPost message submitted late")
						fault.Code = LateSubmission
					}

					faults = append(faults, &fault)
					continue
				}
				// check for missing sectors
				// check for early sector removal
			default:
				continue
			}
		}
	}
	return faults, nil
}

func emptyProofs(msg *types.SignedMessage) bool {
	if len(msg.Params) == 0 {
		return true
	}
	return false
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
