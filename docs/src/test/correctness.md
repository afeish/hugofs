# Test the correctness of the IO

```shell
# use io test to generate some random data, and copy to hugo, to check the correctness of the read and write
# min size is 10m, max size is 50m, batch number is 10
hugo c io test --mins 10m --maxs 50m -b 10
```

```md
+-----+----------------+--------+-------------+-----------+----------------------------------+----------------------------------+---------+
| IDX |      NAME      |  SIZE  | COPIED SIZE | HASH-SIZE |            ORIGIN-MD5            |            COPIED-MD5            | MATCHED |
+-----+----------------+--------+-------------+-----------+----------------------------------+----------------------------------+---------+
|   1 | mock3012304596 | 13 MiB | 13 MiB      | 13 MiB    | f12aa25f75e92f35f6fdbd5eab5ad0c0 | f12aa25f75e92f35f6fdbd5eab5ad0c0 | true    |
+-----+----------------+--------+-------------+-----------+----------------------------------+----------------------------------+---------+
|   2 | mock588137886  | 15 MiB | 15 MiB      | 15 MiB    | fd4c6099211e6d26f3ff05c926bf57a1 | fd4c6099211e6d26f3ff05c926bf57a1 | true    |
+-----+----------------+--------+-------------+-----------+----------------------------------+----------------------------------+---------+
|   3 | mock3631145415 | 18 MiB | 18 MiB      | 18 MiB    | f7dc3512b671af6e7f037f17d5c35cb3 | f7dc3512b671af6e7f037f17d5c35cb3 | true    |
+-----+----------------+--------+-------------+-----------+----------------------------------+----------------------------------+---------+
|   4 | mock1833509894 | 19 MiB | 19 MiB      | 19 MiB    | f2711cf788f2805fa7d236002cb3e0a3 | f2711cf788f2805fa7d236002cb3e0a3 | true    |
+-----+----------------+--------+-------------+-----------+----------------------------------+----------------------------------+---------+
|   5 | mock2978809471 | 21 MiB | 21 MiB      | 21 MiB    | 859a47310ce9411b93ce2662d94faf06 | 859a47310ce9411b93ce2662d94faf06 | true    |
+-----+----------------+--------+-------------+-----------+----------------------------------+----------------------------------+---------+
|   6 | mock833869325  | 24 MiB | 24 MiB      | 24 MiB    | 160cd341016656c4d4f81e6b60c7e1f5 | 160cd341016656c4d4f81e6b60c7e1f5 | true    |
+-----+----------------+--------+-------------+-----------+----------------------------------+----------------------------------+---------+
|   7 | mock2545518204 | 22 MiB | 22 MiB      | 22 MiB    | f3bb478540f646987071d473f4361b5c | f3bb478540f646987071d473f4361b5c | true    |
+-----+----------------+--------+-------------+-----------+----------------------------------+----------------------------------+---------+
|   8 | mock2418381753 | 26 MiB | 26 MiB      | 26 MiB    | 429cab6d72f63e46a61c8a6c99cb3459 | 429cab6d72f63e46a61c8a6c99cb3459 | true    |
+-----+----------------+--------+-------------+-----------+----------------------------------+----------------------------------+---------+
|   9 | mock2597243809 | 46 MiB | 46 MiB      | 46 MiB    | a7078261c89df449db19f853a983a0f6 | a7078261c89df449db19f853a983a0f6 | true    |
+-----+----------------+--------+-------------+-----------+----------------------------------+----------------------------------+---------+
|  10 | mock1416155196 | 45 MiB | 45 MiB      | 45 MiB    | 3b75040dd33dc3b54576a9e61d6ef1af | 3b75040dd33dc3b54576a9e61d6ef1af | true    |
+-----+----------------+--------+-------------+-----------+----------------------------------+----------------------------------+---------+

```
