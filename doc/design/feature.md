# 功能

## 功能

一期：
	- 支持 多 bmc 子网/集群 管理

	- bmc 子网支持 dhcp server，从而实现 bmc 自动发现 

	    * dhcp lease 持久化
	        支持 生产环境的 pvc 存储
	        支持 POC 环境的 本地存储

	    * dhcp 的 underlay 接入
	        支持 hostnetwork 部署， 但不支持单节点部署多实例（健康检查的端口冲突问题）
	        支持 spiderpool/macvlan 部署

		* 支持 dhcp server 中 ip 和 mac 的固化（ 映射关系的修改，需要人工介入 ） 

	- 支持 bmc 手动管理  

- 支持 redfish 的信息获取
    * 基本信息获取

- 支持 redfish 运维
    * 重启
    * pxe 引导
        带内子网上， 需要手动部署一个 PXE 服务（ 包括 dhcp server 和 sftp server）

- 支持 http 代理访问 GUI (不需要)


二期 ？
	- 支持通过 SNMP 获取告警

	- redfish 的 metrics ？

    - 脚本化支持 固件升级 ？


