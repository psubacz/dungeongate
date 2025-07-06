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
- logging
- prometheus metrics     ☐ Add dashboard configuration for common metrics
- stream gameplay
- shared game state for nethack "bones" accoss multiple instances of nethack
- game isolation when player