# Design Documentation Workflow

## Overview

This workflow guides you through creating technical design documents using the template at `<project_root>/server/docs/technical-documentation-template.md`.

## When to Use This Template

Use the design documentation template when:

1. **New Feature Development** - Building significant new functionality that affects multiple components or services
2. **System Changes** - Modifying core infrastructure, database schemas, or API contracts
3. **Performance Optimization** - Making changes that could impact system performance or resource usage
4. **Integration Work** - Connecting with external services or third-party APIs
5. **Complex Bug Fixes** - Fixes that require architectural changes or have broad impact

## When NOT to Use This Template

Skip the design document for:

- Simple bug fixes with obvious solutions
- Minor UI tweaks or copy changes
- Documentation updates
- Dependency upgrades (unless major version with breaking changes)
- Code refactoring that doesn't change behavior

## How to Use

### 1. Copy the Template

```bash
cp server/docs/design_docs_template.md server/docs/usecase/<feature-name>.md
```

### 2. Fill Out Required Sections

| Section | Purpose |
|---------|---------|
| **Document Signature** | Track ownership and link to task |
| **Background / Problem Statement** | Explain the current situation and why change is needed |
| **Goals** | Define measurable success criteria |
| **Non-Goals** | Clarify what is out of scope |
| **Functional Requirements** | Specify technical constraints (RPS, latency, etc.) |
| **Solution Options** | Present 1-2 alternatives with trade-offs |
| **Design** | Include sequence diagrams or flowcharts |
| **Potential Impact** | Identify risks and side effects |
| **Test Plan** | Define how to verify the implementation |
| **Deployment Plan** | Document rollout strategy |
| **Rollback Plan** | Prepare for failure scenarios |
| **Post Deployment** | Define monitoring and alerting |
| **Reviewed by** | Track approvals |

### 3. Review Process

1. Draft the document and share with your team lead
2. Get feedback from Technical Architect and peers
3. Update the document based on review comments
4. Obtain sign-off before implementation begins

### 4. During Implementation

- Reference the design document in PR descriptions
- Update the document if requirements change
- Document any deviations from the original plan

## Best Practices

- **Be specific** - Use concrete numbers, not vague terms like "fast" or "scalable"
- **Show alternatives** - Always present at least one alternative approach
- **Include diagrams** - Visual representations help reviewers understand flow
- **Define metrics** - Specify how you'll measure success
- **Plan for failure** - Always have a rollback strategy

## Example Workflow

```
1. Receive task for new API endpoint
2. Create design doc: server/docs/usecase/add-mountain-scoring-api.md
3. Fill out template sections
4. Share with team for review
5. Address feedback and get approval
6. Implement feature
7. Reference design doc in PR
8. Update doc with any implementation changes
```

## Related Files

- Template: `server/docs/design_docs_template.md`
- Use case docs directory: `server/docs/usecase/`