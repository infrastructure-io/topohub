# 生产部署

## bmc 子网
bmc 组件在 bmc 网段上 启动一个 dhcp 服务器（它可以配置 client 的 gateway 指向 路由器 ），因此 ，该子网的路由器不应该 再启动 dhcp 服务了

## os 子网

pxe 引导问题，在 os 带内子网上， 手动部署一个 PXE 服务（ 包括 dhcp server 和 sftp server）， 

bmc 组件只负责 发送 redfish 指令给 host 开始 pxe 引导  ， 而 bmc 组件上 不负责 dhcp server 和 sftp server

