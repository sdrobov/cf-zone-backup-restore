# Cloudflare Zone DNS Records backup and restore tool
```
USAGE:
cf-zone-backup [global options] command [command options] [arguments...]

COMMANDS:
backup   backup zones
restore  restore zones
help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
--email value, -e value  cf account email address
--key value, -k value    cf account api key
--token value, -t value  cf account api token
--dir value, -d value    backup directory (default: ./)
--url value, -u value    cf api url (default: https://api.cloudflare.com/client/v4/)
--verbose, -v            verbose output (default: false)
--help, -h               show help
```
