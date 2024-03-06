package collection

const Size = 32 * 1024 * 1024 // 32MB

// Collection is a collection of blocks, it will be stored in the cloud storage.
// Then the collection becomes an Object.
//
// Block can be accessed by offset.
// Assume the address of collection in cloud storage is A.
// We can save the relationship like this:
//
//	key: <BlockIndexInFile>:<TxnID>;
//	value: <A>:<BlockIndex>;
//
// Read a object from cloud storage is just read the object from a offset.
type Collection struct{}

// TODO:
// How to trigger the construction of the collection?
// When the physicalVolume achieve a threshold, we can trigger the construction of the collection.
//
// We can pick some blocks which have a low access frequency to construct the collection.
// Then we upload the collection.
