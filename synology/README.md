# Synology SPK Skeleton

这个目录是一套面向当前仓库的 Synology SPK 打包骨架，目标是先把 `OpenSpeedTestX` 的 Go 服务端和静态前端装进一个可手工安装的 `.spk`。

## 包含内容

- `INFO.sh`: 生成 `INFO` 元数据
- `conf/privilege`: DSM 7 所需的权限配置
- `scripts/`: 生命周期脚本，负责启动/停止服务
- `build-spk.sh`: 不依赖 Synology toolkit 的本地打包脚本

## 运行时布局

- 二进制: `/var/packages/openspeedtestx/target/bin/openspeedtestx`
- 前端资源: `/var/packages/openspeedtestx/target/share/openspeedtestx`
- 历史记录: `/var/packages/openspeedtestx/var/history.json`
- 日志目录: `/var/packages/openspeedtestx/var/log`

## 构建示例

构建 x86_64:

```bash
SPK_ARCH=x86_64 SPK_VERSION=1.0.0-0001 ./synology/build-spk.sh
```

构建 armv8:

```bash
SPK_ARCH=armv8 SPK_VERSION=1.0.0-0001 ./synology/build-spk.sh
```

输出文件默认放在 `dist/synology/`。

## 支持的 `SPK_ARCH`

- `x86_64`
- `armv8`
- `armv7`
- `armv5`
- `i686`

也支持一部分 Synology 平台值别名，例如 `apollolake`、`avoton`、`rtd1296`、`armada37xx`。

## 可调参数

- `SPK_VERSION`: DSM 包版本，默认 `1.0.0-0001`
- `SPK_ARCH`: 目标架构，默认 `x86_64`
- `SPK_PORT`: 服务端口，默认 `3000`
- `SPK_OUTPUT_DIR`: 输出目录，默认 `dist/synology`
- `SPK_MAINTAINER`: `INFO` 中的维护者字段，默认 `q000q000`

这套骨架目前走“纯 Go 交叉编译 + 手工组装 `.spk`”路线，适合当前这个纯 Go + 静态资源项目。后续如果你要切换到 Synology 官方 toolkit，`INFO.sh`、`conf/` 和 `scripts/` 可以继续复用。
