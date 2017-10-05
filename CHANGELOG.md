# Release v1.0.4 (2017-XX-YY)

## Major Themes

Support config of External Entities, Host Entities, SFC Entities, and VNF
Entities.  Northbound API's exist to configure each of these.  Also,
a start up YAML file can be used to configure the controller.

The controller maintains its own "database" in ETCD.  The controller "syncs"
its configuration with ETCD and runs a "reconcile" or resyunc procedure to
ensure all configuration is consistent.

If the controller is starterd without a "config" yaml file, then it will use
what it previously had in its database, and will continue to maintain its
database as northbound APIs are received.  If the controller is started with
a config yaml file, then this "new" config is what will be applied and all
"old" entries will be removed.

## Compatibility
Ligato vpp-agent 1.0.4
