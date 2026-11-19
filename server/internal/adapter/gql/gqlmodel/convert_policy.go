package gqlmodel

import (
	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

func ToPolicyAcceptances(as []legal.Acceptance) []*PolicyAcceptance {
	res := make([]*PolicyAcceptance, len(as))
	for i, a := range as {
		res[i] = &PolicyAcceptance{AcceptedAt: a.AcceptedAt, Type: string(a.Type), Version: a.Version.String()}
	}
	return res
}

func ToPolicyRefs(input []*PolicyAcceptanceInput) ([]legal.Ref, error) {
	res := make([]legal.Ref, len(input))
	for i, in := range input {
		t, err := legal.ParseType(in.Type)
		if err != nil {
			return nil, err
		}
		v, err := legal.ParseVersion(in.Version)
		if err != nil {
			return nil, err
		}
		res[i] = legal.Ref{Type: t, Version: v}
	}
	return res, nil
}

func ToPolicyVersions(vs []legal.VersionRecord, lang string) []*PolicyVersion {
	res := make([]*PolicyVersion, len(vs))
	for i, v := range vs {
		pv := &PolicyVersion{
			EffectiveDate: v.EffectiveAt,
			Required:      v.Required,
			Title:         v.Title.Pick(lang),
			Type:          string(v.Type),
			Version:       v.Version.String(),
		}
		if url := v.URL.Pick(lang); url != "" {
			pv.URL = &url
		}
		res[i] = pv
	}
	return res
}
