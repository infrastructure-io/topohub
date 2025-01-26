# e2e

## 部署

在 工程的 test 目录下，创建相关的 E2E 工程代码

具备 test/Makefile 文件 
- test/Makefile 文件中，有 名为 init  的 target  ，  部署一个kubernetes 集群的 kind 环境，其中包括一个 control-plane 和 一个 worker 节点  

- test/Makefile 文件中，有 名为 deploy 的target  支持 部署本工程
第一步，它要使用  工程的 git 的 commit hash 号 作为镜像 tag， 使用 工程 根目录下的 Makefile 来构建所有 测试镜像
第二步，它需要使用 上一步镜像，使用 kind load 把 本地主机上的镜像 加载到 kind 节点上
第三步，它使用 chart 目录中的 chart ，部署本到 kind 环境中, 其中， 其 镜像 tag 使用 工程的 git 的 commit hash 号

- test/Makefile 文件中，有 名为 clean 的 target  支持 卸载 kind  kubernetes 环境 ， 该target 中的命令允许必要的失败，例如 kind 集群不存在等场景


在工程根目录下 的 Makefile 文件中
- 具备 名为 e2e 的 target ，先后 对  test/Makefile 中的 clean、init 、 deploy 调用，完成 kind  kubernetes 环境 和 本工程的部署
- 具备 名为 e2e-clean 的 target ，实现对  test/Makefile 中的 clean target 完成调用

