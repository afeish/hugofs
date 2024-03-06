# MVCC

Once read/write a file, we will open a transaction, and read the latest committed version of the file.

The core the MVCC is a global unique monotonic increasing version number, which is called `GlobalVersion`.

In our situation, the `GlobalVersion` may overflow, so we use multiple `GlobalVersion` to avoid overflow.
Particularly, each `VirtualVolume` will hold a `GloablVersion`.
To make it simple, we call it `VirtualVolumeVersion`.

Then we can define the interface like this:

```protobuf
syntax = "proto3";

service StorageMeta {
  //////////////////////////////////////////////////
  // BeginTransaction will result the transaction_id increase.
  // Make each virtual volume holds a separate transaction to avoid overflow.
  rpc BeginTransaction(BeginTransaction.Request) returns (BeginTransaction.Response);
  // CommitTransaction will commit the transaction to make the change visible.
  // By removing the given transaction id from the active transaction list.
  rpc CommitTransaction(CommitTransaction.Request) returns (CommitTransaction.Response);
  // RollbackTransaction will rollback the transaction, by deleting the changes with the given transaction id.
  rpc RollbackTransaction(RollbackTransaction.Request) returns (RollbackTransaction.Response);
  //////////////////////////////////////////////////
}

// Transaction describes a transaction format.
message Transaction {
  uint64 id = 1;
  enum State {
    INVALID = 0;
    ACTIVE = 1;
    COMMITTED = 2;
    ROLLBACK = 3;
  }
  State state = 2;
  uint64 start_at = 3;
  uint64 end_at = 4;
}

message BeginTransaction {
  message Request {
    // make each virtual volume hold a transaction
    uint64 virtual_volume_id = 1;
  }
  message Response {
    uint64 transaction_id = 1;
  }
}

message CommitTransaction {
  message Request {
    uint64 virtual_volume_id = 1;
    uint64 transaction_id = 2;
  }
  message Response {}
}

message RollbackTransaction {
  message Request {
    uint64 virtual_volume_id = 1;
    uint64 transaction_id = 2;
  }
  message Response {}
}
```

## MVCC structure

There will be three kinds of data structures to support MVCC.

1. crucial monotonic number binding to each VirtualVolume, which is called `VirtualVolumeVersion`.
2. ActiveTxnList, it is also binding to a VirtualVolume
3. RollbackTxnList, it is also binding to a VirtualVolume

Use external lists is to make querying which transaction is active or rollback faster.

## MVCC operation

### Begin a transaction

When we begin a transaction on a VirtualVolume, we will get a `transaction_id` from the Metadata server.
Under the hood, we copy the `InoBlockMap` with the `TransactionID` to the `ActiveTxnList` of the VirtualVolume.

### Commit a transaction

When we commit a transaction, metadata server will remove the transaction from the `ActiveTxnList`,
at the same time, update the original `InoBlockMap`.

### Rollback a transaction

When we roll back a transaction,
we delete the data with the given transaction id from the `ActiveTxnList`.

## Conflict

When we come to a conflict, like 2 transactions trying to write a file and operate the same block,
the second transaction should fail even if it commits early.

## How to use

In my option, we should shield the transaction from the user.
We can make VirtualVolume do this work.

On a write operation, we will open a transaction, and read the latest committed version of the file.
We should call the Transaction on the StorageNode side, clients will not be aware of it.
 

