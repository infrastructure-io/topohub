
# v0.6.0
Welcome to the v0.6.0 release 
Compared with version:v0.5.0, version:v0.6.0 has the following updates.

***

## New Feature

* add label filtering to the Reconcile of secrets : [PR 155](https://github.com/infrastructure-io/topohub/pull/155)

* update redfishstatus reconcile  : [PR 151](https://github.com/infrastructure-io/topohub/pull/151)

* support hostendpoint update : [PR 159](https://github.com/infrastructure-io/topohub/pull/159)

* secret update support : [PR 162](https://github.com/infrastructure-io/topohub/pull/162)

* support dhcp create hostendpoint : [PR 171](https://github.com/infrastructure-io/topohub/pull/171)

* pxe support ipmitools : [PR 175](https://github.com/infrastructure-io/topohub/pull/175)

* Add session pool to support connection reuse : [PR 178](https://github.com/infrastructure-io/topohub/pull/178)

* Redfish status data updates every minute, info data updates every day : [PR 179](https://github.com/infrastructure-io/topohub/pull/179)

* Split the code in the main function by business type : [PR 181](https://github.com/infrastructure-io/topohub/pull/181)

* Accessing SSH client using the session pool : [PR 182](https://github.com/infrastructure-io/topohub/pull/182)

* Add scripts support to build the test chart : [PR 184](https://github.com/infrastructure-io/topohub/pull/184)

* Update Redfish info using a coroutine pool. : [PR 183](https://github.com/infrastructure-io/topohub/pull/183)

* Accessing redfish client using the session pool : [PR 185](https://github.com/infrastructure-io/topohub/pull/185)

* Attempt to update session pool when HostEndpoint is modified : [PR 187](https://github.com/infrastructure-io/topohub/pull/187)

* support secret update : [PR 188](https://github.com/infrastructure-io/topohub/pull/188)



***

## Fix

* Some servers do not support the SimpleStorage interface : [PR 154](https://github.com/infrastructure-io/topohub/pull/154)

* fix startup cache being empty  causes scheduled tasks to not work : [PR 158](https://github.com/infrastructure-io/topohub/pull/158)

* hostEndpoint port updates were not syncing to RedfishStatus  : [PR 161](https://github.com/infrastructure-io/topohub/pull/161)

* fix ssh hostendpoint update type error : [PR 163](https://github.com/infrastructure-io/topohub/pull/163)

* fix redfish client timeout too long : [PR 164](https://github.com/infrastructure-io/topohub/pull/164)

* hostendpoint clustername support change : [PR 166](https://github.com/infrastructure-io/topohub/pull/166)

* fix pxe bootOverride error : [PR 167](https://github.com/infrastructure-io/topohub/pull/167)

* fix the issue with device list information synchronization : [PR 169](https://github.com/infrastructure-io/topohub/pull/169)

* fix sshstatus update error : [PR 172](https://github.com/infrastructure-io/topohub/pull/172)

* Fixed some lint issues in the pkg/subnet/dhcpserver package : [PR 174](https://github.com/infrastructure-io/topohub/pull/174)

* fix redfish npe error : [PR 176](https://github.com/infrastructure-io/topohub/pull/176)

* Fixed the host health check issue : [PR 189](https://github.com/infrastructure-io/topohub/pull/189)

* Fixed the host health check issue 2 : [PR 190](https://github.com/infrastructure-io/topohub/pull/190)

* update deployment affinity : [PR 191](https://github.com/infrastructure-io/topohub/pull/191)

* fix update subnet not work : [PR 192](https://github.com/infrastructure-io/topohub/pull/192)



***

## Totoal 

Pull request number: 37

[ Commits ](https://github.com/infrastructure-io/topohub/compare/v0.5.0...v0.6.0)
