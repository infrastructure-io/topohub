# 文件管理

topohub pod 可使用 pvc 或者 hostPath 来持久化存储数据，默认挂载到 POD 的 `/var/lib/topohub/` 目录下，它包含了如下目录

1. dhcp 目录

  目录中存储了以 subnet 名字命名的 DHCP server 的配置文件、日志、dhcp ip 分配记录的 lease 文件

2. filebrowser 目录

  如果 helm 安装开启了 values.httpServer.enabled 为 true，则会创建此目录，用于持久化 filebrowser 服务的配置数据

3. tftp 目录

  该目录是 dnsmasq 的 tftp 服务的文件目录，其中存储了 PXE 安装 OS 的配置文件

4. http 目录
  
  该目录是 topohub 内置的 http server 的工作目录，主要用于 PXE 装机过程中获取 ISO 镜像文件、

## filebrowser 服务

在安装 topohub 时，helm 安装可开启了 values.httpServer.enabled，会启动 filebrowser 服务，它是一个 http 的文件浏览器，它是为了方便管理员管理 topohub POD 的 `/var/lib/topohub/` 目录下所有文件:

* 例如，在 http 目录中上传 ISO 镜像文件。

filebrowser 服务默认的 IP 地址是 “ topohub pod 所在主机的 IP + 8080 端口”，它的默认登录账户是 admin，密码是 admin，在登录  filebrowser 后，请在管理页面中及时修改密码

filebrowser 默认可参考 [官方文档](https://github.com/filebrowser/filebrowser)

