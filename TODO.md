# Updated TODO.md - Microservice Architecture Migration

## Completed ✅

- ✅ All configs now live in the configs directory
- ✅ Implemented golang-standards/project-layout structure
- ✅ Created microservice architecture with separate services:
  - Auth Service (authentication & authorization)
  - User Service (user management & registration)
  - Game Service (game management & process control)
  - Session Service (TTY sessions & recording)
- ✅ Separated build-time and runtime configurations
- ✅ Kubernetes-ready configuration structure
- ✅ Namespace isolation design (replaces traditional chroot)
- ✅ Stream encryption support in session service
- ✅ External database made optional (SQLite default, PostgreSQL optional)

## In Progress 🚧

### Core Functionality Implementation
- Logging to file
- Container files
- Stream gameplay debugging
- Stream game windows for game messages
- Implement configuation limits for number of connections to ssh for resource constrained systems (number of connecttions and games allowed per server)
- update watch menu to look better
- Increase login attempts when failure to login
- Look into inactivity time and automatic logout
- Automated password reset and account recovery (should require an sshkey or email.)
- Semi-public mode which requires accounts
- Logging implementation
- Prometheus metrics
- Add dashboard template configuration for common metrics
- Game isolation when multiple players are using the
- Shared game state for nethack "bones" accoss multiple server/containers of nethack
- look into https://alt.org/nethack/ integration?
- server scoring per user
- global server scores
