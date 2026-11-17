import type { components } from './schema';
import { apiRequest } from './request';

type Schema = components['schemas'];
type Options = { signal?: AbortSignal };
type PageOptions = Options & { cursor?: string; limit?: number };
type LineageOptions = Options & { usage_cursor?: string; artifact_cursor?: string; limit?: number };
export type ExperiencePolicy = Schema['ExperiencePolicy'];
export type ExperiencePolicySettings = Schema['ExperiencePolicySettings'];
export type ExperienceCapabilities = Schema['ExperienceCapabilities'];

const projectPath = (project: string) => `/projects/${encodeURIComponent(project)}`;
const versionPath = (project: string, version: string) => `${projectPath(project)}/experiences/${encodeURIComponent(version)}`;
function pageQuery(options: PageOptions) {
  const query = new URLSearchParams();
  if (options.cursor) query.set('cursor', options.cursor);
  if (options.limit !== undefined) query.set('limit', String(options.limit));
  return query.size ? `?${query}` : '';
}

export const getExperiencePolicy = (project: string, options: Options = {}) =>
  apiRequest<ExperiencePolicy>(`${projectPath(project)}/experience-policy`, options);
export const saveExperiencePolicy = (project: string, input: Schema['ExperiencePolicyUpdate'], csrf: string, options: Options = {}) =>
  apiRequest<ExperiencePolicy>(`${projectPath(project)}/experience-policy`, { ...options, method: 'PUT', body: input, csrf });
export const getExperienceCapabilities = (project: string, options: Options = {}) =>
  apiRequest<ExperienceCapabilities>(`${projectPath(project)}/experience-capabilities`, options);
export const listExperiences = (project: string, options: PageOptions = {}) =>
  apiRequest<Schema['ExperiencePage']>(`${projectPath(project)}/experiences${pageQuery(options)}`, { signal: options.signal });
export const getExperience = (project: string, version: string, options: Options = {}) =>
  apiRequest<Schema['ExperienceVersion']>(versionPath(project, version), options);
export const listExperienceUsages = (project: string, version: string, options: PageOptions = {}) =>
  apiRequest<Schema['ExperienceUsagePage']>(`${versionPath(project, version)}/usages${pageQuery(options)}`, { signal: options.signal });
export const revokeExperience = (project: string, version: string, reason: string, csrf: string, options: Options = {}) =>
  apiRequest<void>(`${versionPath(project, version)}/revoke`, { ...options, method: 'POST', body: { reason }, csrf });
export const deleteExperience = (project: string, version: string, reason: string, csrf: string, options: Options = {}) =>
  apiRequest<void>(versionPath(project, version), { ...options, method: 'DELETE', body: { reason }, csrf });
export const activateExperience = (project: string, version: string, input: Schema['ExperienceClaimRequest'], csrf: string, options: Options = {}) =>
  apiRequest<Schema['ExperienceVersion']>(`${versionPath(project, version)}/activate`, { ...options, method: 'POST', body: input, csrf });
export const changeExperienceDelivery = (project: string, version: string, input: Schema['ExperienceDeliveryRequest'], csrf: string, options: Options = {}) =>
  apiRequest<Schema['ExperienceVersion']>(`${versionPath(project, version)}/delivery`, { ...options, method: 'POST', body: input, csrf });
export const listEvolutionIntents = (project: string, options: PageOptions = {}) =>
  apiRequest<Schema['EvolutionIntentPage']>(`${projectPath(project)}/evolution/intents${pageQuery(options)}`, { signal: options.signal });
export const getEvolutionIntent = (project: string, intent: string, options: Options = {}) =>
  apiRequest<Schema['EvolutionIntentSummary']>(`${projectPath(project)}/evolution/intents/${encodeURIComponent(intent)}`, options);
export const startEvolutionCandidate = (project: string, input: Schema['EvolutionCandidateRequest'], csrf: string, options: Options = {}) =>
  apiRequest<Schema['EvolutionIntentSummary']>(`${projectPath(project)}/evolution/intents`, { ...options, method: 'POST', body: input, csrf });
export const decideEvolution = (project: string, intent: string, generation: number, csrf: string, options: Options = {}) =>
  apiRequest<Schema['EvolutionDecision']>(`${projectPath(project)}/evolution/intents/${encodeURIComponent(intent)}/decision`, { ...options, method: 'POST', body: { generation }, csrf });
export const promoteEvolution = (project: string, intent: string, generation: number, csrf: string, options: Options = {}) =>
  apiRequest<Schema['EvolutionIntentSummary']>(`${projectPath(project)}/evolution/intents/${encodeURIComponent(intent)}/promote`, { ...options, method: 'POST', body: { generation }, csrf });
export const observeEvolution = (project: string, intent: string, input: Schema['EvolutionObservationRequest'], csrf: string, options: Options = {}) =>
  apiRequest<Schema['EvolutionIntentSummary']>(`${projectPath(project)}/evolution/intents/${encodeURIComponent(intent)}/observe`, { ...options, method: 'POST', body: input, csrf });
export const submitWorkbenchEntry = (project: string, entry: string, input: Schema['WorkbenchEntrySubmission'], key: string, csrf: string, options: Options = {}) =>
  apiRequest<Schema['Submission']>(`${projectPath(project)}/entries/${encodeURIComponent(entry)}/runs`, { ...options, method: 'POST', body: input, key, csrf });
export const listRebuildIntents = (run: string, options: Options = {}) =>
  apiRequest<Schema['ApprovalRebuildIntent'][]>(`/runs/${encodeURIComponent(run)}/rebuild-intents`, options);
export const rebuildInvalidatedRun = (run: string, intent: string, csrf: string, options: Options = {}) =>
  apiRequest<Schema['Submission']>(`/runs/${encodeURIComponent(run)}/rebuild-intents/${encodeURIComponent(intent)}`, { ...options, method: 'POST', body: {}, csrf });
export const observeExperienceMemory = (project: string, version: string, run: string, csrf: string, options: Options = {}) =>
  apiRequest<Schema['ExperienceObservation']>(`${versionPath(project, version)}/observe`, { ...options, method: 'POST', body: { run_id: run }, csrf });
export const getExperienceArtifact = (project: string, artifact: string, options: Options = {}) =>
  apiRequest<Schema['ExperienceArtifactSummary']>(`${projectPath(project)}/experience-artifacts/${encodeURIComponent(artifact)}`, options);
export function getRunExperience(run: string, options: LineageOptions = {}) {
  const query = new URLSearchParams();
  if (options.usage_cursor) query.set('usage_cursor', options.usage_cursor);
  if (options.artifact_cursor) query.set('artifact_cursor', options.artifact_cursor);
  if (options.limit !== undefined) query.set('limit', String(options.limit));
  return apiRequest<Schema['RunExperienceLineage']>(`/runs/${encodeURIComponent(run)}/experience${query.size ? `?${query}` : ''}`, { signal: options.signal });
}
export const rollbackEvolution = (project: string, intent: string, input: Schema['EvolutionRollback'], csrf: string, options: Options = {}) =>
  apiRequest<Schema['EvolutionIntentSummary']>(`${projectPath(project)}/evolution/intents/${encodeURIComponent(intent)}/rollback`, { ...options, method: 'POST', body: input, csrf });
