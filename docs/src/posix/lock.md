POSIX (Portable Operating System Interface) lock is a mechanism used to synchronize access to a shared resource, such as
a file, in a multi-process or multi-threaded environment. The POSIX lock consists of two types of locks: read lock (
shared lock) and write lock (exclusive lock).

The read lock allows multiple processes or threads to access the shared resource concurrently for reading. The write
lock, on the other hand, allows only one process or thread to access the shared resource exclusively for writing,
blocking all other processes or threads from accessing the resource until the write lock is released.

The POSIX lock is implemented using the fcntl system call, which takes a file descriptor and a flock structure as
arguments. The flock structure contains information about the type of lock to be acquired, the starting offset in the
file, and the length of the lock.

To acquire a read lock, the flock structure's l_type field is set to F_RDLCK, and to acquire a write lock, the l_type
field is set to F_WRLCK. The l_whence field is set to SEEK_SET to indicate that the starting offset is relative to the
beginning of the file. The l_start field is set to the starting offset, and the l_len field is set to the length of the
lock.

Once the lock is acquired, other processes or threads attempting to acquire a conflicting lock on the same region of the
file will block until the lock is released.

To release a lock, the fcntl system call is called again with the flock structure's l_type field set to F_UNLCK. This
releases the lock and allows other processes or threads to acquire a lock on the same region of the file.

In summary, the POSIX lock is a mechanism used to synchronize access to a shared resource in a multi-process or
multi-threaded environment. It provides read and write locks, which allow multiple processes or threads to access the
resource concurrently for reading or exclusively for writing. The POSIX lock is implemented using the fcntl system call,
which takes a file descriptor and a flock structure as arguments.
