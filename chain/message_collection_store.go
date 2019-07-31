package chain
import (
	"context"

	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-cid"	

	"github.com/filecoin-project/go-filecoin/types"
)

type MessageCollectionStore struct {
	ipldStore *hamt.CborIpldStore
}

// NewMessageCollectionStore creates and returns a new store
func NewMessageCollectionStore(cst *hamt.CborIpldStore) *MessageCollectionStore {
	return &MessageCollectionStore{
		ipldStore: cst,
	}
}

// LoadMessages loads the signed messages in the collection with cid c from ipld
// storage.
func (ms *MessageCollectionStore) LoadMessages(ctx context.Context, c cid.Cid) ([]*types.SignedMessage, error) {
	// TODO #1324 message collection shouldn't be a slice	
	var out []*types.SignedMessage
	err := ms.ipldStore.Get(ctx, c, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StoreMsgs puts the input signed messages to a collection and then writes
// this collection to ipld storage.  The cid of the collection is returned.
func (ms *MessageCollectionStore) StoreMessages(ctx context.Context, msgs []*types.SignedMessage) (cid.Cid, error) {
	// For now the collection is just a slice (cbor array)
	// TODO #1324 put these messages in a merkelized collection
	return ms.ipldStore.Put(ctx, msgs)
}

// LoadReceipts loads the signed messages in the collection with cid c from ipld
// storage and returns the slice implied by the collection
func (ms *MessageCollectionStore) LoadReceipts(ctx context.Context, c cid.Cid) ([]*types.MessageReceipt, error) {
	var out []*types.MessageReceipt
	err := ms.ipldStore.Get(ctx, c, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StoreReceipts puts the input signed messages to a collection and then writes
// this collection to ipld storage.  The cid of the collection is returned.
func (ms *MessageCollectionStore) StoreReceipts(ctx context.Context, receipts []*types.MessageReceipt) (cid.Cid, error) {
	// For now the collection is just a slice (cbor array)
	// TODO #1324 put these messages in a merkelized collection
	return ms.ipldStore.Put(ctx, receipts)
}


