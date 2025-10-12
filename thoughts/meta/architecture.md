# Architecture Documentation

## Overview

The `thoughts/architecture/` directory contains the foundational design documents that guide your project's development. These documents serve as the source of truth for architectural decisions and help AI agents understand your system's design principles.

## Core Architecture Documents

### overview.md
A high-level outline of the entire architecture, providing:
- Synopsis of each architectural document
- How documents relate to each other
- Quick reference for navigating the architecture

### system-architecture.md
Deep dive into technical infrastructure:
- Programming languages and their usage
- Frameworks and libraries employed
- Core infrastructure components
- Build and deployment tooling
- Development environment setup

### domain-model.md
Business logic and feature design:
- Core domain concepts and entities
- Business rules and constraints
- Feature specifications
- Data relationships and workflows
- User interaction patterns

### testing-strategy.md
Comprehensive testing approach:
- Unit testing conventions
- Integration testing patterns
- End-to-end testing scenarios
- Performance testing requirements
- Test data management

### development-workflow.md
Process and methodology:
- Development phases for tickets
- Code review process
- Branching strategy
- CI/CD pipeline stages
- Architecture change protocols

### persistence.md
Data storage and management:
- Database schemas and migrations
- Caching strategies
- File storage systems
- Search indices
- Data backup and recovery

## Optional Architecture Components

### api-design.md
External interface specifications:
- REST/GraphQL endpoint designs
- Authentication and authorization
- Rate limiting and quotas
- API versioning strategy
- Request/response formats

### cli-design.md
Command-line interface:
- Command structure and syntax
- Configuration management
- Output formatting
- Error handling patterns

### event-bus.md
Asynchronous communication:
- Event types and schemas
- Publishing and subscription patterns
- Event routing and filtering
- Error handling and retries
- Event sourcing (if applicable)

## Best Practices

### 1. Keep Documents Current
- Update architecture docs when making significant changes
- Document decisions and trade-offs
- Include dates for major revisions

### 2. Be Specific
- Use concrete examples
- Reference actual code locations
- Include diagrams where helpful

### 3. Document Constraints
- Technical limitations
- Business requirements
- Performance targets
- Security requirements

### 4. Explain the "Why"
- Rationale behind decisions
- Alternatives considered
- Trade-offs accepted

## How AI Agents Use Architecture Docs

When you run commands like `/research` or `/plan`, the agents:

1. **Discover Context**: Search architecture docs to understand system design
2. **Follow Patterns**: Identify established patterns to maintain consistency
3. **Respect Constraints**: Work within documented limitations
4. **Make Informed Decisions**: Use architectural principles to guide implementation

## Creating Architecture Documents

### Initial Setup
When starting a new project:
1. Create `thoughts/architecture/` directory
2. Start with `overview.md` and `system-architecture.md`
3. Add other documents as the system grows

### Document Template
```markdown
# [Component Name] Architecture

## Overview
Brief description of the component's purpose and role in the system.

## Design Principles
- Principle 1: Explanation
- Principle 2: Explanation

## Components
### [Subcomponent 1]
Description and responsibilities

### [Subcomponent 2]
Description and responsibilities

## Data Flow
How data moves through this component

## Integration Points
How this component connects with others

## Constraints and Limitations
- Technical constraints
- Business rules
- Performance requirements

## Future Considerations
Planned improvements or known technical debt
```

## Maintaining Architecture Docs

### Regular Reviews
- Review quarterly or after major features
- Update to reflect actual implementation
- Archive outdated decisions

### Team Collaboration
- Document decisions from design discussions
- Include stakeholder requirements
- Maintain change log for major updates

### Version Control
- Commit architecture changes with code
- Use meaningful commit messages
- Tag major architecture versions

## Examples

### Good Architecture Documentation
- Specific: "Use PostgreSQL 14+ with JSONB for flexible schema"
- Actionable: "All API endpoints must return within 200ms"
- Current: "As of 2024-01, we use React 18 with TypeScript"

### Poor Architecture Documentation
- Vague: "Use a database"
- Outdated: References deprecated technologies
- Missing context: No explanation of decisions

## Related Documentation
- [Thoughts Directory Structure](./thoughts.md)
- [Development Workflow](./workflow.md)
