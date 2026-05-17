# Contributing to Scouter

Thank you for your interest in contributing to Scouter. We welcome contributions that improve the quality, performance, and functionality of the tool.

## Engineering Standards

To maintain a high-quality codebase, we adhere to the following principles:

1. **Clear Communication**: When reporting issues or proposing changes, provide clear context and technical details.
2. **Verified Code**: All modifications must include relevant tests. Ensure that `go test ./...` passes before submitting a contribution.
3. **Clean Implementation**: Follow Go 1.25+ idioms and maintain a clean, modular architecture. Avoid unnecessary complexity and prioritize readability.

## Development Workflow

1. **Setup**: Fork the repository and create a descriptive feature branch.
2. **Environment**: Ensure you are using Go 1.25 or higher.
3. **Verification**: Run `make build` and `make test` to verify your environment and changes.
4. **Commits**: Use conventional commit messages (e.g., `feat:`, `fix:`, `chore:`).

## Pull Request Guidelines

1. **Focused Scope**: Each pull request should address a single concern or feature.
2. **Documentation**: Update relevant documentation if your change introduces new features or modifies existing behavior.
3. **Review Process**: All contributions will undergo a technical review to ensure alignment with project standards and architectural integrity.

---
*We value contributions that promote technical excellence and maintainable design.*
