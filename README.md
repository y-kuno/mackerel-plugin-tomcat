# mackerel-plugin-tomcat [![Build Status](https://travis-ci.org/y-kuno/mackerel-plugin-tomcat.svg?branch=master)](https://travis-ci.org/y-kuno/mackerel-plugin-tomcat)

Tomcat plugin for mackerel.io agent. This repository releases an artifact to Github Releases, which satisfy the format for mkr plugin installer.

## Install

```shell
mkr plugin install y-kuno/mackerel-plugin-tomcat 
```

## Synopsis

```shell
mackerel-plugin-tomcat [-host=<host>] [-port=<port>] [-user=<user>] [-password=<password>] [-metric-key-prefix=<prefix>]
```

## Example of mackerel-agent.conf

```
[plugin.metrics.tomcat]
command = "/path/to/mackerel-plugin-tomcat"
```

## Documents

* [Server Status](http://tomcat.apache.org/tomcat-8.0-doc/manager-howto.html#Server_Status)
