# njulogin

本地门槛（每个 commit 前）：`make check`；发版前与改依赖后加跑 `make release-check`（含 `govulncheck`）。

文档用英文写。哪份管什么：

- 改行为（`cmd/`、`tools/`）：先读 [SPEC.md](SPEC.md)，它的 Authority 一节列出全部规范文档与优先级；
  命令、flag、退出码看 [docs/cli-contract.md](docs/cli-contract.md)，凭据与远程传输看 [docs/security.md](docs/security.md)。
- 改流程、加依赖、要不要做安全评审：[docs/development-contract.md](docs/development-contract.md)。
- 在哪台机器上干什么：[docs/development-topology.md](docs/development-topology.md)。
- 打包、tag、Release、tap：[docs/release-readiness.md](docs/release-readiness.md)；每一步发布都要 owner 单独授权。
- 交给另一个 agent 评审：[docs/review-handoff.md](docs/review-handoff.md)。
- 测试夹具：[docs/testing-fixtures.md](docs/testing-fixtures.md)。
