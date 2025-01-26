# develop

## steps

1. 创建 kind 开发环境
    ```
    # 本地构建镜像部署环境
    make images
    make e2e

    或者使用已经发行 ghcr 在线版本镜像
    make e2e -e VERSION=v0.3.0

    或者使用已经发行国内在线版本镜像
    make e2e -e VERSION=v0.3.0 -e REGISTRY=ghcr.m.daocloud.io/infrastructure-io


    ```

2. 清理环境 `make e2e-clean`

## issue

1. 对于老的 BMC 系统，它的 tls 版本很低，证书套件很老，导致 gofish 无法正常建立链接

