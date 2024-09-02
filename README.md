# FSD (In Development)
FSD is a modular file system daemon that communicates over a message-passing protocol. Message passing producers and consumers can be added (support coming soon) to extend the functionality beyond what it currently does. FSD exposes metadata about the entire filesystem (starting from the `rootPath`) via a REST api on `http://localhost:16000` by default. It tracks all file updates and reports useful characteristics like disk usage, file age, etc, that are not readily available to applications on remote hosts. All metadata is stored locally in sqlite, but this data is *ephemeral*, meaning that it can be wiped under arbitrary situations (since it can be trivially regenerated).
