## Description
Briefly describe the changes in this PR.

## Type of Change
- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to change)
- [ ] Documentation update
- [ ] Performance improvement

## Related Issues
Closes #(issue)

## Changes Made
- Item 1
- Item 2
- Item 3

## Testing
- [ ] I have tested these changes locally
- [ ] Added/updated unit tests
- [ ] Added/updated integration tests
- [ ] All tests pass: `go test -race ./...`

## Checklist
- [ ] My code follows the code style of this project
- [ ] I have updated the documentation accordingly
- [ ] I have added tests for my changes
- [ ] All new and existing tests pass
- [ ] My commits follow the conventional commit format

## Verification
```bash
go test -race -cover ./...
golangci-lint run ./...
docker build -t renovate-scheduler:test .
```

## Screenshots (if applicable)
Add any screenshots or gifs showing the changes.

## Additional Notes
Any additional context or notes about this PR.
