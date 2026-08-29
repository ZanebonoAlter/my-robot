package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"syntopica-backend/internal/platform/logging"
)

// referenceRoleInjectionCap bounds the total "分析方法参考" appendix at ~4k
// characters (design D5; counted in runes so CJK content isn't triple-penalized
// by byte length). Overflow drops WHOLE entries (never slices an entry's
// markdown mid-block) and logs the truncation for auditability.
const referenceRoleInjectionCap = 4000

const referenceRoleHeader = "\n\n---\n【分析方法参考】（以下为可借鉴的分析方法论，只给方法，不给任何事实/结论）\n"

// referenceRoleAppendix builds the methodology appendix from enabled reference
// roles (design D5). Queried fresh on EVERY call so enable/disable takes effect
// on the next orchestration without restart. Returns "" when no roles are
// enabled (or the read fails) — callers append nothing and the prompt stays
// byte-identical to the no-feature state (M7.4).
func (o *OrchestratorService) referenceRoleAppendix(ctx context.Context) string {
	if o.repo == nil {
		return ""
	}
	roles, err := o.repo.ListEnabledReferenceRoles(ctx)
	if err != nil {
		logging.Warnf("reference roles: list enabled failed, skip injection: %v", err)
		return ""
	}
	if len(roles) == 0 {
		return ""
	}

	var sb strings.Builder
	total := utf8.RuneCountInString(referenceRoleHeader)
	dropped := 0
	for _, r := range roles {
		title := r.Title
		if title == "" {
			title = r.Name
		}
		entry := fmt.Sprintf("## %s（%s）\n%s\n", title, r.Name, r.Content)
		if total+utf8.RuneCountInString(entry) > referenceRoleInjectionCap {
			dropped++
			continue
		}
		sb.WriteString(entry)
		total += utf8.RuneCountInString(entry)
	}
	if sb.Len() == 0 {
		// Nothing fit under the cap — inject nothing rather than a bare header.
		logging.Warnf("reference roles: all %d entries exceed injection cap %d, skip injection", len(roles), referenceRoleInjectionCap)
		return ""
	}
	if dropped > 0 {
		logging.Infof("reference roles: %d entries dropped to fit %d-char injection cap", dropped, referenceRoleInjectionCap)
	}
	return referenceRoleHeader + sb.String()
}

// ReferenceRoleAppendixForTest exposes the private appendix builder to the
// external test package.
func (o *OrchestratorService) ReferenceRoleAppendixForTest(ctx context.Context) string {
	return o.referenceRoleAppendix(ctx)
}

// ReferenceRoleInjectionCapForTest exposes the injection cap constant.
func ReferenceRoleInjectionCapForTest() int { return referenceRoleInjectionCap }
