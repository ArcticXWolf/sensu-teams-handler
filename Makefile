build-snapshot:
	goreleaser build --snapshot --rm-dist

test: build-snapshot
	cat testing/test-event.json | dist/sensu-teams-handler_linux_amd64_v1/bin/sensu-teams-handler