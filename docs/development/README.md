# Development Guides

This directory contains developer operational workflows, coding standards, and governance policies for engineers and AI agents working on this repository:

- **[Developer Workflow & Makefile Guide](developer-workflow.md)**:
  Step-by-step lifecycle from first-time environment setup (`make setup`), local Docker dependencies (`make dev-up`), and database migrations (`make migrate-up`), to day-to-day coding, formatting (`make fmt`), strict linting (`make lint`), and local testing (`make test-race`, `make test-all`). Includes a complete Makefile reference table and troubleshooting FAQ.
- **[Go Implementation Conventions](go-conventions.md)**:
  Normative Go idioms, SOLID principles, CQRS boundaries, error handling, strict service encapsulation and the Parameter Object pattern (`*Params` + `New*Service`), quantitative struct receiver sizing rules ($\le 64$B cache line / register ABI vs $> 64$B pointer receiver), and table-driven unit test policies.
- **[Repository Governance](repository-governance.md)**:
  Harness-neutral Specification-Driven Delivery (SDD) protocol, task claims, collision avoidance across parallel worktrees, and verification gates.

