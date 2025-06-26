
# v0.5.0
Welcome to the v0.5.0 release 
Compared with version:v0.4.0, version:v0.5.0 has the following updates.

***

## New Feature

* integrate scripts to http server : [PR 68](https://github.com/infrastructure-io/topohub/pull/68)

* crd changed: show information : [PR 73](https://github.com/infrastructure-io/topohub/pull/73)

* ztp : [PR 97](https://github.com/infrastructure-io/topohub/pull/97)

* Support retrieving resetTypes from the ResetActionInfo URL interface : [PR 105](https://github.com/infrastructure-io/topohub/pull/105)

* Add SSHStatus CRD definition : [PR 137](https://github.com/infrastructure-io/topohub/pull/137)

* Support SSH protocol to retrieve server information : [PR 138](https://github.com/infrastructure-io/topohub/pull/138)

* DHCP supports IP allocation only for bound MAC addresses : [PR 147](https://github.com/infrastructure-io/topohub/pull/147)

* helm support pprof and pyroscope : [PR 150](https://github.com/infrastructure-io/topohub/pull/150)



***

## Fix

* fix: data race : [PR 60](https://github.com/infrastructure-io/topohub/pull/60)

* fix: dhcp comfigmap : [PR 61](https://github.com/infrastructure-io/topohub/pull/61)

* fix: it takes long to synchronize hoststatus when lots of hostendpoints are created   : [PR 82](https://github.com/infrastructure-io/topohub/pull/82)

* fix the incomplete log information : [PR 100](https://github.com/infrastructure-io/topohub/pull/100)

* fix  conflict between  subnet network and  host network : [PR 102](https://github.com/infrastructure-io/topohub/pull/102)

* fix IPRange check error : [PR 104](https://github.com/infrastructure-io/topohub/pull/104)

* fix: it doest not update subnet status when dhcp lease file is changed : [PR 108](https://github.com/infrastructure-io/topohub/pull/108)

* add BindingIP  mac check : [PR 126](https://github.com/infrastructure-io/topohub/pull/126)

* Fix DHCP assigned IP binding not  work : [PR 143](https://github.com/infrastructure-io/topohub/pull/143)

* Resolve the occasional failure issue with PXE installation on Lenovo servers : [PR 144](https://github.com/infrastructure-io/topohub/pull/144)

* Fix subnet crd DhcpExpireTime time not updating : [PR 145](https://github.com/infrastructure-io/topohub/pull/145)

* Update  SSH status type and ensure it is consistent with Redfish : [PR 148](https://github.com/infrastructure-io/topohub/pull/148)



***

## Totoal 

Pull request number: 38

[ Commits ](https://github.com/infrastructure-io/topohub/compare/v0.4.0...v0.5.0)
