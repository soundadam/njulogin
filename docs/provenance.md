# Source provenance

The standalone source was recovered from the locally committed
`adam-network-tools` implementation identified by commit `558472b`
(`feat: add njulogin remote auth workflow`) and its later uncommitted fixes
recorded in the same local development session.

The implementation was originally designed after reviewing the MIT-licensed
[`Do1e/NJUlogin`](https://github.com/Do1e/NJUlogin) project. During extraction
on 2026-07-27, upstream `v5.1.1` at commit
`c138f1025e0944f423c52d1e7837e402db0ea73d` was inspected for current project
identity and license. No upstream Python source, model, or binary was copied
into this repository.

The extracted product intentionally excludes unrelated network, proxy, VPN,
and server-management code from the former workspace.
