# FSD v1

## Overview
FSD is a modular file system daemon that communicates over a message-passing protocol. Message passing producers and consumers can be added (support coming soon) to extend the functionality beyond what it currently does. FSD exposes metadata about the entire filesystem (starting from the `rootPath`) via a REST api on `http://localhost:16000` by default. It tracks all file updates and reports useful characteristics like disk usage, file age, etc, that are not readily available to applications on remote hosts. All metadata is stored locally in sqlite, but this data is *ephemeral*, meaning that it can be wiped under arbitrary situations (since it can be trivially regenerated).

## Goals
1. FSD Should be fast
2. FSD Should be easy
3. FSD Should be extensible
4. FSD Should be reproducible.

FSD is a safe system for processing results from file changes anywhere on your file system, and giving access to those data points as time-series data. The API endpoints provide historical information, but it's up to the ingestion point to derive meaning from that data. All of the compaction intervals are configurable to ensure that data does not balloon in memory. 

## Getting Started
The recommended way to get started is to first great a dedicated user for `fsd`. This is provided via a helper script in the repository, but it's pretty easy, so you should just do it yourself. You should then copy `defaultconfig.toml` into your `~/.fsd/config.toml` and edit the `rootPath` to point to your desired root directory. You may also update any additional configs here as needed. The CLI also supports passing in configs via command line flags, but this is not recommended for production use, as they will not be saved between runs.

From there, you can easily build the project via `./scripts/build` and then install via `./scripts/install`. This will create a systemd service unit file in `/etc/systemd/system/fsd.service` that you can then start the service with `sudo systemctl start fsd@user.service` where `user` is the user that you want to start `fsd` as.

### Upgrading
Upgrades are simple, you just get the new binary and move it to the same location as before, then restart the service with `sudo systemctl restart fsd@user.service`. Reproducibility is important, if you need to just blow away your prior state, including the database and config, re-creating everything should be as easy as starting the application. The guiding tenant is being self-contained.
