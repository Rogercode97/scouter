# ⚔️ Contributing to Scouter (Wave 8.9)

Thank you for your interest in contributing to **Scouter**, the Sovereign Truth Kernel for AI Agents.

## ⚖️ The Supreme Mandates
By contributing, you agree to abide by the Hakaishin Engineering Standards:
1. **Absolute Signal**: No chitchat in PRs or issues. State the problem, the RCA (Root Cause Analysis), and the solution.
2. **Empirical Absolute**: Code without tests (`go test ./...`) does not exist. All fixes must include verification.
3. **Zero Slop**: Keep functions small, architecture clean, and respect the Go 1.25+ idioms. No `any`, no naked panics.

## 🚀 Development Workflow
1. Fork the repository and create a feature branch.
2. Install dependencies and ensure you are using Go 1.25+.
3. Run `make build` and `make test`.
4. Run the validation suite: `scouter strike` (if available) or `go vet ./...`.
5. Commit using conventional commits (`feat:`, `fix:`, `chore:`).

## 🛡️ Pull Request Process
1. Ensure your PR is focused on a single concern.
2. Include the output of your tests.
3. If your change affects architecture, provide the `scouter_impact` analysis.
4. Wait for the Supreme Judgment.

*Results over process. Impact over dogma. Hakai.*
