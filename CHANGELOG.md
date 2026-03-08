# Changelog

## 0.2.0

- `cloudru-community_placement_group` resource added

## 0.1.5

- Bugfix: DNS Server's state after creation doesn't store the comuted ipAddress, which leads to the resource being updated on every apply. Now the state is properly updated with the computed ipAddress after creation.
- Extended README.md with the provider example pointing to the fact of using the provider in pair with the official one.

## 0.1.4

- Stabilization of async operations on resources via REST API of cloud.ru

## 0.1.3

FEATURES:

- cloudru-community provider
- `cloudru-community_vpc` resource added
- `cloudru-community_dns_server` resource added
- `cloudru-community_subnet` resource added (managed via Compute API)
- `cloudru-community_vpcs` data source added
