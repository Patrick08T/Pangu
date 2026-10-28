package research

import "errors"

func (s *Session) checkEvidence(a reportArgs) error {
	external, web, project := false, false, false
	for _, claim := range a.Claims {
		if claim.Kind != "fact" {
			continue
		}
		for _, item := range claim.Evidence {
			source := s.sources[item.SourceID]
			if item.SourceID == "input" || !source.Read {
				continue
			}
			external = true
			web = web || source.URL != ""
			project = project || source.URL == "" && source.ArtifactID == ""
		}
	}
	if (s.input.Node.Mode == "research" || s.input.Node.Mode == "competition") && !external {
		return errors.New("证据不足：调研或竞品报告必须包含实际读取资料支持的事实，只有用户目标或未知项不能算完成")
	}
	if len(s.brief.PublicQueries)+len(s.brief.URLs) > 0 && !web {
		return errors.New("证据不足：已要求联网资料，报告必须引用实际读取的网页原文；搜索结果标题不算正文证据")
	}
	if len(s.brief.Documents) > 0 && !project {
		return errors.New("证据不足：已要求项目资料，报告必须引用实际读取的项目原文；请核对授权路径和 Git 基准")
	}
	return nil
}
