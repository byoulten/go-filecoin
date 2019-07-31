package chain_test

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestOneBlock(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	t1 := builder.AppendOn(genesis, 1)
	t1H, err := t1.Height()
	require.NoError(t, err)
	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(t1.Key(), types.Uint64(t1H), peer.ID(""), true)))

	verifyTip(t, store, t1, t1.At(0).StateRoot)
	verifyHead(t, store, t1)
}

func TestMultiBlockTip(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	tip := builder.AppendOn(genesis, 2)
	tipH, err := tip.Height()
	require.NoError(t, err)
	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(tip.Key(), types.Uint64(tipH), peer.ID(""), true)))

	verifyTip(t, store, tip, builder.StateForKey(tip.Key()))
	verifyHead(t, store, tip)
}

func TestTipSetIncremental(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	t1 := builder.AppendOn(genesis, 1)
	t1H, err := t1.Height()
	require.NoError(t, err)

	t2 := builder.AppendOn(genesis, 1)
	t2H, err := t2.Height()
	require.NoError(t, err)

	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(t1.Key(), types.Uint64(t1H), peer.ID(""), true)))

	verifyTip(t, store, t1, builder.StateForKey(t1.Key()))
	verifyHead(t, store, t1)

	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(t2.Key(), types.Uint64(t2H), peer.ID(""), true)))
	verifyTip(t, store, t2, builder.StateForKey(t2.Key()))

	merged := types.RequireNewTipSet(t, t1.At(0), t2.At(0))
	verifyTip(t, store, merged, builder.StateForKey(merged.Key()))
	verifyHead(t, store, merged)
}

func TestChainIncremental(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	t1 := builder.AppendOn(genesis, 2)
	t1H, err := t1.Height()
	require.NoError(t, err)

	t2 := builder.AppendOn(t1, 3)
	t2H, err := t2.Height()
	require.NoError(t, err)

	t3 := builder.AppendOn(t2, 1)
	t3H, err := t3.Height()
	require.NoError(t, err)

	t4 := builder.AppendOn(t3, 2)
	t4H, err := t4.Height()
	require.NoError(t, err)

	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(t1.Key(), types.Uint64(t1H), peer.ID(""), true)))
	verifyTip(t, store, t1, builder.StateForKey(t1.Key()))
	verifyHead(t, store, t1)

	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(t2.Key(), types.Uint64(t2H), peer.ID(""), true)))
	verifyTip(t, store, t2, builder.StateForKey(t2.Key()))
	verifyHead(t, store, t2)

	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(t3.Key(), types.Uint64(t3H), peer.ID(""), true)))
	verifyTip(t, store, t3, builder.StateForKey(t3.Key()))
	verifyHead(t, store, t3)

	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(t4.Key(), types.Uint64(t4H), peer.ID(""), true)))
	verifyTip(t, store, t4, builder.StateForKey(t4.Key()))
	verifyHead(t, store, t4)
}

func TestChainJump(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	t1 := builder.AppendOn(genesis, 2)
	t2 := builder.AppendOn(t1, 3)
	t3 := builder.AppendOn(t2, 1)
	t4 := builder.AppendOn(t3, 2)

	t4H, err := t4.Height()
	require.NoError(t, err)

	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(t4.Key(), types.Uint64(t4H), peer.ID(""), true)))
	verifyTip(t, store, t1, builder.StateForKey(t1.Key()))
	verifyTip(t, store, t2, builder.StateForKey(t2.Key()))
	verifyTip(t, store, t3, builder.StateForKey(t3.Key()))
	verifyTip(t, store, t4, builder.StateForKey(t4.Key()))
	verifyHead(t, store, t4)
}

func TestIgnoreLightFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	forkbase := builder.AppendOn(genesis, 1)
	forkHead := builder.AppendOn(forkbase, 1)
	fhH, err := forkHead.Height()
	require.NoError(t, err)

	t1 := builder.AppendOn(forkbase, 1)
	t2 := builder.AppendOn(t1, 1)
	t3 := builder.AppendOn(t2, 1)
	t4 := builder.AppendOn(t3, 1)
	t4H, err := t4.Height()
	require.NoError(t, err)

	// Sync heaviest branch first.
	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(t4.Key(), types.Uint64(t4H), peer.ID(""), true)))
	verifyTip(t, store, t4, builder.StateForKey(t4.Key()))
	verifyHead(t, store, t4)

	// Lighter fork is processed but not change head.
	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(forkHead.Key(), types.Uint64(fhH), peer.ID(""), true)))
	verifyTip(t, store, forkHead, builder.StateForKey(forkHead.Key()))
	verifyHead(t, store, t4)
}

func TestAcceptHeavierFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	forkbase := builder.AppendOn(genesis, 1)

	main1 := builder.AppendOn(forkbase, 1)
	main2 := builder.AppendOn(main1, 1)
	main3 := builder.AppendOn(main2, 1)
	main4 := builder.AppendOn(main3, 1)
	m4H, err := main4.Height()
	require.NoError(t, err)

	// Fork is heavier with more blocks, despite shorter (with default fake weighing function
	// from FakeStateEvaluator).
	fork1 := builder.AppendOn(forkbase, 3)
	fork2 := builder.AppendOn(fork1, 1)
	fork3 := builder.AppendOn(fork2, 1)
	f3H, err := fork3.Height()
	require.NoError(t, err)

	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(main4.Key(), types.Uint64(m4H), peer.ID(""), true)))
	verifyTip(t, store, main4, builder.StateForKey(main4.Key()))
	verifyHead(t, store, main4)

	// Heavier fork updates hea3
	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(fork3.Key(), types.Uint64(f3H), peer.ID(""), true)))
	verifyTip(t, store, fork1, builder.StateForKey(fork1.Key()))
	verifyTip(t, store, fork2, builder.StateForKey(fork2.Key()))
	verifyTip(t, store, fork3, builder.StateForKey(fork3.Key()))
	verifyHead(t, store, fork3)
}

func TestFarFutureTipsets(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	t.Run("accepts when syncing", func(t *testing.T) {
		builder, store, _ := setup(ctx, t)
		genesis := builder.RequireTipSet(store.GetHead())
		farHead := builder.AppendManyOn(chain.FinalityLimit+1, genesis)
		fhH, err := farHead.Height()
		require.NoError(t, err)

		syncer := chain.NewSyncer(&chain.FakeStateEvaluator{}, store, builder, chain.Syncing)
		assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(farHead.Key(), types.Uint64(fhH), peer.ID(""), true)))
	})

	t.Run("rejects when caught up", func(t *testing.T) {
		builder, store, _ := setup(ctx, t)
		genesis := builder.RequireTipSet(store.GetHead())
		farHead := builder.AppendManyOn(chain.FinalityLimit+1, genesis)
		fhH, err := farHead.Height()
		require.NoError(t, err)

		syncer := chain.NewSyncer(&chain.FakeStateEvaluator{}, store, builder, chain.CaughtUp)
		// TODO(frrist): you will want to set reusted to false here
		err = syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(farHead.Key(), types.Uint64(fhH), peer.ID(""), true))
		assert.Error(t, err)
	})
}

func TestNoUncessesaryFetch(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	head := builder.AppendManyOn(4, genesis)
	hH, err := head.Height()
	require.NoError(t, err)
	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(head.Key(), types.Uint64(hH), peer.ID(""), true)))

	// A new syncer unable to fetch blocks from the network can handle a tipset that's already
	// in the store and linked to genesis.
	emptyFetcher := chain.NewBuilder(t, address.Undef)
	newSyncer := chain.NewSyncer(&chain.FakeStateEvaluator{}, store, emptyFetcher, chain.Syncing)
	assert.NoError(t, newSyncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(head.Key(), types.Uint64(hH), peer.ID(""), true)))
}

// Syncer must track state of subsets of parent tipsets tracked in the store
// when they are the ancestor in a chain.  This is in order to maintain the
// invariant that the aggregate state of the  parents of the base of a collected chain
// is kept in the store.  This invariant allows chains built on subsets of
// tracked tipsets to be handled correctly.
// This test tests that the syncer stores the state of such a base tipset of a collected chain,
// i.e. a subset of an existing tipset in the store.
//
// Ex: {A1, A2} -> {B1, B2, B3} in store to start
// {B1, B2} -> {C1, C2} chain 1 input to syncer
// C1 -> D1 chain 2 input to syncer
//
// The last operation will fail if the state of subset {B1, B2} is not
// kept in the store because syncing C1 requires retrieving parent state.
func TestSubsetParent(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	// Set up chain with {A1, A2} -> {B1, B2, B3}
	tipA1A2 := builder.AppendOn(genesis, 2)
	tipB1B2B3 := builder.AppendOn(tipA1A2, 3)
	tbH, err := tipB1B2B3.Height()
	require.NoError(t, err)
	require.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(tipB1B2B3.Key(), types.Uint64(tbH), peer.ID(""), true)))

	// Sync one tipset with a parent equal to a subset of an existing
	// tipset in the store: {B1, B2} -> {C1, C2}
	tipB1B2 := types.RequireNewTipSet(t, tipB1B2B3.At(0), tipB1B2B3.At(1))
	tipC1C2 := builder.AppendOn(tipB1B2, 2)
	tcH, err := tipC1C2.Height()
	require.NoError(t, err)

	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(tipC1C2.Key(), types.Uint64(tcH), peer.ID(""), true)))

	// Sync another tipset with a parent equal to a subset of the tipset
	// just synced: C1 -> D1
	tipC1 := types.RequireNewTipSet(t, tipC1C2.At(0))
	tipD1OnC1 := builder.AppendOn(tipC1, 1)
	tdc1H, err := tipD1OnC1.Height()
	require.NoError(t, err)
	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(tipD1OnC1.Key(), types.Uint64(tdc1H), peer.ID(""), true)))

	// A full parent also works fine: {C1, C2} -> D1
	tipD1OnC1C2 := builder.AppendOn(tipC1C2, 1)
	tdc2c2H, err := tipD1OnC1C2.Height()
	require.NoError(t, err)
	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(tipD1OnC1C2.Key(), types.Uint64(tdc2c2H), peer.ID(""), true)))
}

// Check that the syncer correctly adds widened chain ancestors to the store.
func TestWidenChainAncestor(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	link1 := builder.AppendOn(genesis, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.AppendOn(link3, 2)
	l4H, err := link4.Height()
	require.NoError(t, err)

	// Build another block with parents link1, but not included in link2.
	link2Alt := builder.AppendOn(link1, 1)
	// Build a tipset containing one block from link2, plus this new sibling.
	link2UnionSubset := types.RequireNewTipSet(t, link2.At(0), link2Alt.At(0))
	lusH, err := link2UnionSubset.Height()
	require.NoError(t, err)

	// Sync the subset of link2 first
	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(link2UnionSubset.Key(), types.Uint64(lusH), peer.ID(""), true)))
	verifyTip(t, store, link2UnionSubset, builder.StateForKey(link2UnionSubset.Key()))
	verifyHead(t, store, link2UnionSubset)

	// Sync chain with head at link4
	require.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(link4.Key(), types.Uint64(l4H), peer.ID(""), true)))
	verifyTip(t, store, link4, builder.StateForKey(link4.Key()))
	verifyHead(t, store, link4)

	// Check that the widened tipset (link2UnionSubset U link2) is tracked
	link2Union := types.RequireNewTipSet(t, link2.At(0), link2.At(1), link2.At(2), link2Alt.At(0))
	verifyTip(t, store, link2Union, builder.StateForKey(link2Union.Key()))
}

// Syncer finds a heaviest tipset by combining blocks from the ancestors of a
// chain and blocks already in the store.
//
// A guide to this test -- the point is that sometimes when merging chains the syncer
// will find a new heaviest tipset that is not the head of either chain.  The syncer
// should correctly set this tipset as the head.
//
// From above we have the test-chain:
// genesis -> (link1blk1, link1blk2) -> (link2blk1, link2blk2, link2blk3) -> link3blk1 -> (link4blk1, link4blk2)
//
// Now we introduce a disjoint fork on top of link1
// genesis -> (link1blk1, link1blk2) -> (forklink2blk1, forklink2blk2, forklink2blk3, forklink3blk4) -> forklink3blk1
//
// When all blocks contribute equally to weight:
// So, the weight of the  head of the test chain =
//   W(link1) + 3 + 1 + 2 = W(link1) + 6 = 8
// and the weight of the head of the fork chain =
//   W(link1) + 4 + 1 = W(link1) + 5 = 7
// and the weight of the union of link2 of both branches (a valid tipset) is
//   W(link1) + 7 = 9
//
// Therefore the syncer should set the head of the store to the union of the links..
func TestHeaviestIsWidenedAncestor(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	link1 := builder.AppendOn(genesis, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.AppendOn(link3, 2)
	l4H, err := link4.Height()
	require.NoError(t, err)

	forkLink2 := builder.AppendOn(link1, 4)
	forkLink3 := builder.AppendOn(forkLink2, 1)
	flH, err := forkLink3.Height()
	require.NoError(t, err)

	// Sync main chain
	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(link4.Key(), types.Uint64(l4H), peer.ID(""), true)))

	// Sync fork chain
	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(forkLink3.Key(), types.Uint64(flH), peer.ID(""), true)))

	// Assert that widened chain is the new head
	wideBlocks := link2.ToSlice()
	wideBlocks = append(wideBlocks, forkLink2.ToSlice()...)
	wideTs := types.RequireNewTipSet(t, wideBlocks...)

	verifyTip(t, store, wideTs, builder.ComputeState(wideTs))
	verifyHead(t, store, wideTs)
}

func TestBlocksNotATipSetRejected(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	b1 := builder.AppendBlockOn(genesis)
	b2 := builder.AppendBlockOnBlocks(b1)

	badKey := types.NewTipSetKey(b1.Cid(), b2.Cid())
	err := syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(badKey, b1.Height, peer.ID(""), true))
	assert.Error(t, err)

	_, err = store.GetTipSet(badKey)
	assert.Error(t, err) // Not present
}

func TestBlockNotLinkedRejected(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	// Set up a parallel builder from which the syncer cannot fetch.
	// The two builders are expected to produce exactly the same blocks from the same sequence
	// of calls.
	shadowBuilder := chain.NewBuilder(t, address.Undef)
	gen2 := types.RequireNewTipSet(t, shadowBuilder.AppendBlockOnBlocks())
	require.True(t, genesis.Equals(gen2))

	// The syncer fails to fetch this block so cannot sync it.
	b1 := shadowBuilder.AppendOn(genesis, 1)
	b1H, err := b1.Height()
	require.NoError(t, err)
	assert.Error(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(b1.Key(), types.Uint64(b1H), peer.ID(""), true)))

	// Make the same block available from the syncer's builder
	builder.AppendBlockOn(genesis)
	assert.NoError(t, syncer.HandleNewTipset(ctx, chain.NewPeerHeadInfo(b1.Key(), types.Uint64(b1H), peer.ID(""), true)))
}

///// Set-up /////

// Initializes a chain builder, store and syncer.
// The chain builder has a single genesis block, which is set as the head of the store.
func setup(ctx context.Context, t *testing.T) (*chain.Builder, *chain.Store, *chain.Syncer) {
	builder := chain.NewBuilder(t, address.Undef)
	genesis := builder.NewGenesis()
	genStateRoot, err := builder.GetTipSetStateRoot(genesis.Key())
	require.NoError(t, err)

	store := chain.NewStore(repo.NewInMemoryRepo().ChainDatastore(), hamt.NewCborStore(), &state.TreeStateLoader{}, genesis.At(0).Cid())
	// Initialize chainStore store genesis state and tipset as head.
	require.NoError(t, store.PutTipSetAndState(ctx, &chain.TipSetAndState{genStateRoot, genesis}))
	require.NoError(t, store.SetHead(ctx, genesis))

	// Note: the chain builder is passed as the fetcher, from which blocks may be requested, but
	// *not* as the store, to which the syncer must ensure to put blocks.
	eval := &chain.FakeStateEvaluator{}
	syncer := chain.NewSyncer(eval, store, builder, chain.Syncing)

	return builder, store, syncer
}

///// Verification helpers /////

// Sub-interface of the store used for verification.
type syncStoreReader interface {
	GetHead() types.TipSetKey
	GetTipSet(types.TipSetKey) (types.TipSet, error)
	GetTipSetStateRoot(tsKey types.TipSetKey) (cid.Cid, error)
	GetTipSetAndStatesByParentsAndHeight(types.TipSetKey, uint64) ([]*chain.TipSetAndState, error)
}

// Verifies that a tipset and associated state root are stored in the chain store.
func verifyTip(t *testing.T, store syncStoreReader, tip types.TipSet, stateRoot cid.Cid) {
	foundTip, err := store.GetTipSet(tip.Key())
	require.NoError(t, err)
	assert.Equal(t, tip, foundTip)

	foundState, err := store.GetTipSetStateRoot(tip.Key())
	require.NoError(t, err)
	assert.Equal(t, stateRoot, foundState)

	parent, err := tip.Parents()
	assert.NoError(t, err)
	h, err := tip.Height()
	assert.NoError(t, err)
	childTsasSlice, err := store.GetTipSetAndStatesByParentsAndHeight(parent, h)
	assert.NoError(t, err)
	assert.True(t, containsTipSet(childTsasSlice, tip))
}

// Verifies that the store's head is as expected.
func verifyHead(t *testing.T, store syncStoreReader, head types.TipSet) {
	headTipSet, err := store.GetTipSet(store.GetHead())
	require.NoError(t, err)
	assert.Equal(t, head, headTipSet)
}

func containsTipSet(tsasSlice []*chain.TipSetAndState, ts types.TipSet) bool {
	for _, tsas := range tsasSlice {
		if tsas.TipSet.String() == ts.String() { //bingo
			return true
		}
	}
	return false
}
