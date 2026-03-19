# Scout Implementation Status

**Plan:** `docs/superpowers/plans/2026-03-19-scout-implementation-plan.md`
**Last updated:** 2026-03-19

---

## Foundation (Task 1) — PENDING
- [ ] Schema migration (8 new columns on leads)
- [ ] New tables (lead_artifacts, interactions, content_posts, content_backlog)
- [ ] Pipeline definitions (updated job/freelance + new referral/networking)
- [ ] ValidateTransition enforcement in server.go
- [ ] knownPipelines update in analytics.go
- [ ] Tests passing, make verify green

## Batch 1 — Hour 1 (Tasks 2-7) — PENDING
- [ ] Pipeline runner (runner.go + job phases)
- [ ] ResumeTailor + FreelanceScore
- [ ] NetworkingDraft + WeeklyNetworkingBrief
- [ ] ContentSeriesGen + HookWriter + ContentTopicGen
- [ ] ExpertApplication + CallPrepBrief
- [ ] TierClassifier + DreamCompanies
- [ ] All merged, make verify green

## Batch 2 — Hour 2 (Tasks 8-13) — PENDING
- [ ] SOWGenerator + ContractFollowUp + CaseStudyDraft
- [ ] ConsultingFollowUp + AdvisoryProposal + ProjectProposal + ConsultingUpsellEvaluator
- [ ] ThreadConverter + SubstackExpander + ReactiveContentGen + EngagementReply
- [ ] ContentMetrics + LinkedInUpdate + GitHubREADMEGen
- [ ] ProfileAudit + TestimonialRequest + PinRecommendation
- [ ] ContractUpsellDetector + Runner Wiring (all pipeline phases)
- [ ] All merged, make verify green

## Batch 3 — Hour 3 (Tasks 14-16) — PENDING
- [ ] Priority Queue tab + Gate Framework
- [ ] Jobs + Freelance + Networking Gate UIs
- [ ] Content + Consulting + Contracts + Profile Gate UIs + Metrics
- [ ] All merged, tsc clean, make verify green

## Hour 4 (Tasks 17-19) — PENDING
- [ ] Integration testing (make build + make verify)
- [ ] First real run (TheirStack sweep + AI outputs review)
- [ ] Ship (merge to master)

---

## Tool Inventory (35 total)

| # | Tool | Status | Batch | Agent |
|---|---|---|---|---|
| 1 | ResumeMatch | ✅ Existing | — | — |
| 2 | CoverLetter | ✅ Existing | — | — |
| 3 | ColdOutreach | ✅ Existing | — | — |
| 4 | SalaryLookup | ✅ Existing | — | — |
| 5 | ProposalGen | ✅ Existing | — | — |
| 6 | ReferralFinder | ✅ Existing | — | — |
| 7 | CompanyPitch | ✅ Existing | — | — |
| 8 | ResumeTailor | ⏳ Pending | 1 | Agent 2 |
| 9 | FreelanceScore | ⏳ Pending | 1 | Agent 2 |
| 10 | NetworkingDraft | ⏳ Pending | 1 | Agent 3 |
| 11 | WeeklyNetworkingBrief | ⏳ Pending | 1 | Agent 3 |
| 12 | ContentSeriesGen | ⏳ Pending | 1 | Agent 4 |
| 13 | HookWriter | ⏳ Pending | 1 | Agent 4 |
| 14 | ContentTopicGen | ⏳ Pending | 1 | Agent 4 |
| 15 | ExpertApplication | ⏳ Pending | 1 | Agent 5 |
| 16 | CallPrepBrief | ⏳ Pending | 1 | Agent 5 |
| 17 | TierClassifier | ⏳ Pending | 1 | Agent 6 |
| 18 | SOWGenerator | ⏳ Pending | 2 | Agent 7 |
| 19 | ContractFollowUp | ⏳ Pending | 2 | Agent 7 |
| 20 | CaseStudyDraft | ⏳ Pending | 2 | Agent 7 |
| 21 | ConsultingFollowUp | ⏳ Pending | 2 | Agent 8 |
| 22 | AdvisoryProposal | ⏳ Pending | 2 | Agent 8 |
| 23 | ProjectProposal | ⏳ Pending | 2 | Agent 8 |
| 24 | ConsultingUpsellEvaluator | ⏳ Pending | 2 | Agent 8 |
| 25 | ThreadConverter | ⏳ Pending | 2 | Agent 9 |
| 26 | SubstackExpander | ⏳ Pending | 2 | Agent 9 |
| 27 | ReactiveContentGen | ⏳ Pending | 2 | Agent 9 |
| 28 | EngagementReply | ⏳ Pending | 2 | Agent 9 |
| 29 | ContentMetrics | ⏳ Pending | 2 | Agent 10 |
| 30 | LinkedInUpdate | ⏳ Pending | 2 | Agent 10 |
| 31 | GitHubREADMEGen | ⏳ Pending | 2 | Agent 10 |
| 32 | ProfileAudit | ⏳ Pending | 2 | Agent 11 |
| 33 | TestimonialRequest | ⏳ Pending | 2 | Agent 11 |
| 34 | PinRecommendation | ⏳ Pending | 2 | Agent 11 |
| 35 | ContractUpsellDetector | ⏳ Pending | 2 | Agent 12 |
