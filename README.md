# Overview

Live O Results is a pair of daemons that provide an on-the-day results service for orienteering events.

The daemons work from the "Live HTML" output from AutoDownload. 

`filewatcher` daemon monitors the index.html file from AutoDownload. Each time this updates, the daemon decodes the xhtml and stores in an internal structure. The daemon can then transmit the results data to any registered `resultserver`s.

`resultserver` is intended to be installed on an internet accessible host. Once running, you register the host:port with the `filewatcher` daemon (currently manually through the filewatcher's manager ui), and from there, results will be sent from `filewatcher` to `resultserver`. 

Event participants and other interested parties can then view the live results by accessing the daemon through a web browser at host:port/results.

It is expected that the `resultserver` is accessible on port 80, but this may be implemented using proxypass or equivalent in web server config.


