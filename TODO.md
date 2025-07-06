# Updated TODO.md - Microservice Architecture Migration

## Completed ✅

- ✅ All configs now live in the configs directory
- ✅ Implemented golang-standards/project-layout structure
- ✅ Session Service fully implemented (SSH server, HTTP handlers, TTY management)
- ✅ Separated build-time and runtime configurations
- ✅ Kubernetes-ready configuration structure
- ✅ External database made optional (SQLite default, PostgreSQL optional)
- ✅ Namespace isolation configuration structure designed

## In Progress 🚧

### Microservice Implementation
- Auth Service implementation (currently stub with health endpoint only)
- User Service implementation (partial - service layer exists, needs HTTP handlers)
- Game Service implementation (currently stub with health endpoint only)
- Stream encryption implementation (currently stub returning unencrypted data)
- Namespace isolation implementation (Linux syscalls and container integration)

### Core Functionality Implementation
- Logging to file, template in `pkg/log/log.go`
- Stream gameplay clear buffers once user exits, only need live game play
- Stream game windows for game messages
- Implement configuation limits for number of connections to ssh for resource constrained systems (number of connecttions and games allowed per server)
- update watch menu to look better
- Increase login attempts when failure to login
- Look into inactivity time and automatic logout
- Automated password reset and account recovery (should require an sshkey or email.)
- Semi-public mode which requires accounts
- Logging implementation
- Prometheus metrics, not all are displaying as expected. 
- Add dashboard template configuration for common metrics
- Game isolation when multiple players are using the
- Shared game state for nethack "bones" accoss multiple server/containers of nethack
- look into https://alt.org/nethack/ integration?
- server scoring per user
- global server scores
- login failure. notify user that user name doest exist
- login failure. notify user that password is incorrect
- autosave on exit or ctrl-c fromm game. make it a user option. enabled by default
- edit message when exiting that game was saved and display the hash of the save.
- Container files for each service
- helm charts