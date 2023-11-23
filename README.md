1. Download relevant binary from the releases page.

2. Run binary with options

[API Tokens documentation](https://docs.gamebench.net/docs/web-dashboard/api-tokens/)

API username: API username will be the email addressed used to log into the web dashboard.

Session ID: Session ID can be copied from the URL when accessing the session in the web dashboard.

The below example is using the linux binary. Please change the command to use the relevant binary for your OS.

```
./gb-offline-logs-linux-amd64 -web-dashboard-url <Web dashboard URL> -api-username <API username> -api-token <API token> -company-id <Company ID> -session-id <Session ID>
```

3. View generated HTML in browser
