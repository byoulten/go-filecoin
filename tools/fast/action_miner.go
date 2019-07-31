package fast

import (
	"context"
	"math/big"
	"strconv"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/commands"
	"github.com/filecoin-project/go-filecoin/porcelain"
)

// MinerCreate runs the `miner create` command against the filecoin process
func (f *Filecoin) MinerCreate(ctx context.Context, collateral *big.Int, options ...ActionOption) (address.Address, error) {
	var out commands.MinerCreateResult

	sCollateral := collateral.String()

	args := []string{"go-filecoin", "miner", "create"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, sCollateral)

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return address.Undef, err
	}

	return out.Address, nil
}

// MinerUpdatePeerid runs the `miner update-peerid` command against the filecoin process
func (f *Filecoin) MinerUpdatePeerid(ctx context.Context, minerAddr address.Address, pid peer.ID, options ...ActionOption) (cid.Cid, error) {
	var out commands.MinerUpdatePeerIDResult

	args := []string{"go-filecoin", "miner", "update-peerid"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, minerAddr.String(), pid.Pretty())

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return cid.Undef, err
	}

	return out.Cid, nil
}

// MinerOwner runs the `miner owner` command against the filecoin process
func (f *Filecoin) MinerOwner(ctx context.Context, minerAddr address.Address) (address.Address, error) {
	var out address.Address

	sMinerAddr := minerAddr.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "miner", "owner", sMinerAddr); err != nil {
		return address.Undef, err
	}

	return out, nil
}

// MinerPower runs the `miner power` command against the filecoin process and returns the result
// as two *big.Ints
func (f *Filecoin) MinerPower(ctx context.Context, minerAddr address.Address) (*big.Int, *big.Int, error) {
	var out string

	sMinerAddr := minerAddr.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "miner", "power", sMinerAddr); err != nil {
		return big.NewInt(0), big.NewInt(0), err
	}

	powers := strings.Split(out, " / ")
	minerPwr, err := strconv.ParseInt(powers[0], 10, 64)
	if err != nil {
		return big.NewInt(0), big.NewInt(0), err
	}
	totalPwr, err := strconv.ParseInt(powers[1], 10, 64)
	if err != nil {
		return big.NewInt(0), big.NewInt(0), err
	}
	return big.NewInt(minerPwr), big.NewInt(totalPwr), nil
}

// MinerSetPrice runs the `miner set-price` command against the filecoin process
func (f *Filecoin) MinerSetPrice(ctx context.Context, fil *big.Float, expiry *big.Int, options ...ActionOption) (*porcelain.MinerSetPriceResponse, error) {
	var out commands.MinerSetPriceResult

	sExpiry := expiry.String()
	sFil := fil.Text('f', -1)

	args := []string{"go-filecoin", "miner", "set-price"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, sFil, sExpiry)

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out.MinerSetPriceResponse, nil
}

// MinerProvingPeriod runs the `miner proving-period` command against the filecoin process
func (f *Filecoin) MinerProvingPeriod(ctx context.Context, miner address.Address) (commands.MinerProvingPeriodResult, error) {
	var out commands.MinerProvingPeriodResult

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "miner", "proving-period", miner.String()); err != nil {
		return commands.MinerProvingPeriodResult{}, err
	}

	return out, nil
}
