# Pull Request Guidelines

When creating PRs, follow the template in ` <project_root>/.github/pull_request_template.md`.

## Required Sections

### Why
- Explain the reason for the change (not "what" but "why")
- If you have a Notion ticket, add the ticket ID to the PR title
- The Notion GitHub App will autolink the ticket and your pull request

### Checklist
Always verify and check off:
- [ ] Verified backward compatibility related to feature modifications (if not compatible, reported deployment notes to the next release owner)
- [ ] Confirmed backward compatibility for migrations
- [ ] Verified that no personally identifiable information (PII) is included in any values that may be displayed

## PR Description Format

```markdown
## Why

[Explain why this change is needed - the problem being solved]

[If applicable, include technical details about the root cause and solution]

## Checklist

- [x] Verified backward compatibility related to feature modifications
- [x] Confirmed backward compatibility for migrations
- [x] Verified that no PII is included in displayed values
```

## Best Practices

- Write PR descriptions in English
- Link to related issues or Notion tickets in the title
- Include links to related code in other repositories when applicable
- For cross-repository changes, link to relevant files in reearth-accounts, reearth-visualizer, etc.
