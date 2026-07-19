# Contributing to Sync Pad

感谢你的兴趣！以下是参与贡献的指南。

## 本地运行

```bash
git clone https://github.com/NoelJudeNoel/sync-pad.git
cd sync-pad
make run
```

打开 http://localhost:8080/s/ 即可使用。

## 代码风格

- Go: 使用 `gofmt` 格式化，`make fmt` 一键处理
- CSS: 变量统一在 `base.css` 的 `:root` 中定义
- JS: 保持零依赖，ES6+

## 提交 PR

1. Fork 仓库
2. 创建 feature 分支 (`git checkout -b feature/xxx`)
3. 提交变更 (`git commit -m "feat: xxx"`)
4. 推送到分支
5. 开启 PR 到 main 分支

## 提交规范

- `feat:` 新功能
- `fix:` 修复 bug
- `refactor:` 重构
- `docs:` 文档
- `test:` 测试
