# GC of metadata part

It is a background task running on the metadata server.

But it should also cloud be triggered manually.

So the crucial logic should be implemented independently.

At present, we only need to delete the data of the transaction which is `Abroad`.

1. Txn

   There will be some steal `Transaction` if the server crashed when doing a transaction.
    - `Active`: We make the `Active` transaction as `Rollback` and delete the data.
    - `Rollback`: We delete the data and remove the transaction from the `AbortedTxnList`.

## Caller

1. Period

   Set a period to run the GC task.
2. The threshold

   When the data size is larger than the threshold, we run the GC task.
3. Triggered

   We can also trigger the GC task manually.
