# Version3 of the hugo Storage Design

In this version, I will describe a storage design that is based on MVCC
to fit the needs of the hugo project.

## Abstract

We split files into fixed-size `blocks`, and store them on the `StorageNode`.
To build the relationship between the `blocks` and the `files`, we introduce
a new concept called `InoBlockMap` records how file data is organized.

To achieve multiple versions, we use an `MVCC`-like mechanism to manage the
`blocks` and implement the `snapshot` mechanism at the same time.

To achieve high availability, we introduce a new concept called `VirtualVolume`
to handle the replication and load balance of the `StorageNode`.

To manage massive `blocks` on each `StorageNode`, we introduce a new concept
called `PhysicalVolume`.

## Introduce

I divide this book in two parts, the first part is the `Data Part`, which is
responsible for handling persist data.

The Second part is the `Metadata Part`, which is responsible for recording the
metadata of the `Data Part`.

## Implement

Most of the concepts are defined already, check the `storage package` and I document
some important concepts here, so we can implement them now.

Feel free to ask me if you have any questions.



