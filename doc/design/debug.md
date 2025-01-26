# 调试

## 本地代码验证

1. 请在本地 主机上，尝试  go build  ， 对 cmd 下所有的 golang 程序进行 编译，确保 能够编译成功，如果不成功，请自动修复相关 编译错误，以确保 能够编译成功 。 期间，如果要进行 go mod 库同步，请配置如下环境变量 

    export GOPROXY=https://goproxy.io

    构建调试成功后，请删除本地构建出来的一些临时文件和二进制，不要产生工程垃圾

2. 请在本地主机上，尝试使用 helm template 命令调试  chart 目录下的代码，确保没有 bug 并进行自动修复 ，能够 helm 渲染成功

3. 对所有产生的代表变更，请进行必要的 相应的 markdown 文档变更 

4 . 请在本机上，基于工程当前的分支， 对修改进行 git 提交 ， 完成 git add, git commit, git push
   其中，git commit 中的 message 请使用英文

5. 调试镜像构建
    可以通过 ssh 方式 登录 如下 linux 调试主机 
    10.20.1.20 ， 用户名 root ， 密码 lanwz

    请在 调试主机 上，尝试使用如下命令来获取 代码：
    cd /home/welan
    [ ! -d "bmc" ] || gi clone  https://github.com/infrastructure-io/topohub.git
    cd bmc

    进入工程目录后，切换到 上一步骤 中的分支。
    请基于本工程的 makefile 中的能力，在 调试主机上构建所有的工程镜像 ， 并不断地修改本地工程的代码，并再次尝试 本地提交、调试主机 git pull 同步代码，确保镜像构建成功

5. 尝试 kubernetes 环境部署和调试


## 远程代码验证

代码位于远程主机上，本地 windsurf 打开的工程就是远程主机上的代码，所以本地编辑器任何的代码变更，都是直接生效到远程主机

因此，使用如下方式 进行代码 

1 可以通过 ssh 方式 登录 如下 linux 调试主机 
    10.20.1.20 ， 用户名 root ， 密码 lanwz
    切换到 工程目录下  /home/welan/bmc

2. 在远程主机上，尝试使用 工程根目录下的 makefile   ， 对 cmd 下所有的 golang 程序进行 编译，确保 能够编译成功，如果不成功，请自动修复相关 编译错误，以确保 能够编译成功 。 期间，如果要进行 go mod 库同步，请配置如下环境变量 

    export GOPROXY=https://goproxy.io

    修复完 bug 后，需要运行  go mod vendor 同步
    构建调试成功后，请删除构建出来的一些临时文件和二进制，不要产生工程垃圾

3. 在远程主机上，尝试使用 helm template 命令调试  chart 目录下的代码，确保没有 bug 并进行自动修复 ，能够 helm 渲染成功

4. 在远程主机上， 对所有产生的代码变更，请进行必要的 相应的 markdown 文档变更 

5. 在远程主机上，调试镜像构建
    请基于本工程的 makefile 中的能力，在 调试主机上构建所有的工程镜像 ， 并不断地修改代码，确保镜像构建成功

 
## e2e 测试

代码位于远程主机上，本地 windsurf 打开的工程就是远程主机上的代码，所以本地编辑器任何的代码变更，都是直接生效到远程主机

因此，使用如下方式 进行代码 

1 可以通过 sshpass 方式 登录 如下 linux 调试主机 
   10.20.1.20 ， 用户名 root ， 密码 lanwz
    切换到 工程目录下  /home/welan/bmc

2 在远程主机上，构建镜像构建 make images
    请基于本工程的 makefile 中的能力，在 调试主机上构建所有的工程镜像 ，在尽可能不改变 其它逻辑和优化情况下， 不断地修改代码修复bug，确保镜像构建成功
    期间，如果要进行 go mod 库同步，请配置如下环境变量 
    export GOPROXY=https://goproxy.io
    修复完 bug 后，需要运行  go mod vendor 同步

3. 在远程主机上，尝试使用 make e2e 来验证 环境能够部署成功，其中，重点验证 本工程 的 chart 能够在 环境中部署成功 ，pod 运行正常 ，其中的 日志没有 异常
期间，如果有bug ，在尽可能不改变 其它逻辑和优化情况下， 不断地修改代码修复bug ， 修复完 bug 后，需要运行  go mod vendor 同步

## 提交

1. 请执行  go mod vendor 同步代码 ， 并自动修复 bug 
 期间，如果要进行 go mod 库同步，请配置如下环境变量 
    export GOPROXY=https://goproxy.io

2  基于工程当前的分支， 对修改进行 git 提交 ， 完成 git add, git commit, git push
   其中，git commit 中的 message 请使用英文

