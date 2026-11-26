package workspace

// SubjectKind is what a client call acts for (ADR-0002 decisions 5 to 7).
type SubjectKind uint8

const (
	// SubjectNone is a client token without X-Reearth-Act-As: the call is
	// anonymous (decision 7.1).
	SubjectNone SubjectKind = iota
	SubjectIntegration
	SubjectUser
)

// Subject is a parsed X-Reearth-Act-As value. The zero value is SubjectNone.
// Its fields are private so that only the constructors below build one: a
// user subject always has a user ID and never an integration ID.
type Subject struct {
	kind        SubjectKind
	integration IntegrationID
	user        UserID
}

func IntegrationSubject(iid IntegrationID) Subject {
	return Subject{kind: SubjectIntegration, integration: iid}
}

func UserSubject(uid UserID) Subject {
	return Subject{kind: SubjectUser, user: uid}
}

func (s Subject) Kind() SubjectKind { return s.kind }

func (s Subject) Integration() (IntegrationID, bool) {
	return s.integration, s.kind == SubjectIntegration
}

func (s Subject) User() (UserID, bool) {
	return s.user, s.kind == SubjectUser
}

// String renders "integration:<id>", "user:<id>" or "none". It is the only
// form of the subject that reaches logs (decision 8).
func (s Subject) String() string {
	switch s.kind {
	case SubjectIntegration:
		return "integration:" + s.integration.String()
	case SubjectUser:
		return "user:" + s.user.String()
	default:
		return "none"
	}
}

// Client names the registered client whose token made a call. It holds no
// token and no email.
type Client struct {
	ID string
	// Name is the registered name. Its charset is checked when the registry
	// is decoded, so it is safe to log.
	Name string
}
