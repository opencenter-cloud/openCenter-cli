# BDD Test Fix Roadmap

```
┌─────────────────────────────────────────────────────────────────────┐
│                    BDD TEST FIX ROADMAP                             │
│                                                                     │
│  Current: 99/145 passing (68%)  →  Target: 145/145 passing (100%) │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ WEEK 1: HIGH PRIORITY FIXES                                        │
└─────────────────────────────────────────────────────────────────────┘

Day 1-2: PHASE 1 - Test Fixtures
┌──────────────────────────────────────┐
│ ✓ Find fixtures with enabled services│
│ ✓ Disable cert-manager               │
│ ✓ Disable keycloak                   │
│ ✓ Run validation tests               │
│                                      │
│ Impact: +5 scenarios                 │
│ Time: 2 hours                        │
│ Pass Rate: 68% → 72%                 │
└──────────────────────────────────────┘

Day 3-5: PHASE 2 - Path Structure Updates
┌──────────────────────────────────────┐
│ ✓ Update cluster_steps.go            │
│ ✓ Update config_steps.go             │
│ ✓ Update organization_steps.go       │
│ ✓ Create helper functions            │
│ ✓ Run organization tests             │
│                                      │
│ Impact: +25 scenarios                │
│ Time: 4-6 hours                      │
│ Pass Rate: 72% → 89%                 │
└──────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ WEEK 2: MEDIUM/LOW PRIORITY FIXES                                  │
└─────────────────────────────────────────────────────────────────────┘

Day 1: PHASE 3 - Output Format Updates
┌──────────────────────────────────────┐
│ ✓ Update list command expectations   │
│ ✓ Update help text checks            │
│ ✓ Run CLI tests                      │
│                                      │
│ Impact: +3 scenarios                 │
│ Time: 1-2 hours                      │
│ Pass Rate: 89% → 91%                 │
└──────────────────────────────────────┘

Day 2-3: PHASE 4 - GitOps Setup Fixes
┌──────────────────────────────────────┐
│ ✓ Debug template rendering           │
│ ✓ Fix force flag behavior            │
│ ✓ Verify file overwrites             │
│ ✓ Run GitOps tests                   │
│                                      │
│ Impact: +5 scenarios                 │
│ Time: 3-4 hours                      │
│ Pass Rate: 91% → 93%                 │
└──────────────────────────────────────┘

Day 4: PHASE 5 - VRRP Validation
┌──────────────────────────────────────┐
│ ✓ Review VRRP validation logic       │
│ ✓ Debug validation failures          │
│ ✓ Fix or update expectations         │
│ ✓ Run VRRP tests                     │
│                                      │
│ Impact: +5 scenarios                 │
│ Time: 2-3 hours                      │
│ Pass Rate: 93% → 96%                 │
└──────────────────────────────────────┘

Day 5: PHASE 6 - Bootstrap/Git Fixes
┌──────────────────────────────────────┐
│ ✓ Fix git initialization             │
│ ✓ Fix remote push operations         │
│ ✓ Run bootstrap tests                │
│ ✓ Final verification                 │
│                                      │
│ Impact: +2 scenarios                 │
│ Time: 2 hours                        │
│ Pass Rate: 96% → 100% ✅             │
└──────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ PROGRESS TRACKING                                                   │
└─────────────────────────────────────────────────────────────────────┘

Pass Rate Progress:
68% ████████████████░░░░░░░░ (Start)
72% █████████████████░░░░░░░ (After Phase 1)
89% █████████████████████░░░ (After Phase 2) ← Quick Win Milestone
91% ██████████████████████░░ (After Phase 3)
93% ██████████████████████░░ (After Phase 4)
96% ███████████████████████░ (After Phase 5)
100% ████████████████████████ (After Phase 6) ← Target

Scenarios Fixed:
0   ░░░░░░░░░░░░░░░░░░░░░░░░ (Start)
5   ██░░░░░░░░░░░░░░░░░░░░░░ (After Phase 1)
30  █████████████░░░░░░░░░░░ (After Phase 2) ← Quick Win Milestone
33  ██████████████░░░░░░░░░░ (After Phase 3)
38  ████████████████░░░░░░░░ (After Phase 4)
43  ██████████████████░░░░░░ (After Phase 5)
46  ████████████████████████ (After Phase 6) ← Target

┌─────────────────────────────────────────────────────────────────────┐
│ DECISION POINTS                                                     │
└─────────────────────────────────────────────────────────────────────┘

After Phase 2 (89% pass rate):
┌──────────────────────────────────────┐
│ Option A: Continue to 100%           │
│   - Complete all phases              │
│   - 1 more week                      │
│   - Full test coverage               │
│                                      │
│ Option B: Ship with 89%              │
│   - Document remaining issues        │
│   - Create follow-up tickets         │
│   - Focus on other priorities        │
└──────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ RISK MITIGATION                                                     │
└─────────────────────────────────────────────────────────────────────┘

Checkpoint After Each Phase:
┌──────────────────────────────────────┐
│ 1. Run full test suite               │
│ 2. Verify no regressions             │
│ 3. Commit changes                    │
│ 4. Update progress tracker           │
│ 5. Proceed to next phase             │
└──────────────────────────────────────┘

Rollback Strategy:
┌──────────────────────────────────────┐
│ If issues arise:                     │
│ - Revert to last checkpoint          │
│ - Analyze failure                    │
│ - Adjust approach                    │
│ - Retry with new strategy            │
└──────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ RESOURCE ALLOCATION                                                 │
└─────────────────────────────────────────────────────────────────────┘

Developer Time:
┌────────────────────────────────────────────────────────────┐
│ Week 1 (High Priority)                                     │
│ ████████████████████████████████████░░░░░░░░░░░░░░░░░░░░░░│
│ 6-8 hours                                                  │
│                                                            │
│ Week 2 (Medium/Low Priority)                               │
│ ████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░│
│ 8-11 hours                                                 │
│                                                            │
│ Total: 14-19 hours (~2-3 days)                             │
└────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ SUCCESS METRICS                                                     │
└─────────────────────────────────────────────────────────────────────┘

Minimum Success (Quick Win):
✓ Phase 1 & 2 complete
✓ 89% pass rate (129/145)
✓ 1 day effort
✓ High-priority issues resolved

Target Success (Full Fix):
✓ All phases complete
✓ 100% pass rate (145/145)
✓ 2-3 days effort
✓ All issues resolved

Stretch Goals:
✓ Additional test coverage
✓ Performance improvements
✓ Documentation updates
✓ CI/CD integration

┌─────────────────────────────────────────────────────────────────────┐
│ NEXT ACTIONS                                                        │
└─────────────────────────────────────────────────────────────────────┘

Immediate (Today):
[ ] Review and approve this roadmap
[ ] Assign developer resources
[ ] Set up progress tracking
[ ] Create Phase 1 branch

This Week:
[ ] Complete Phase 1 (Test Fixtures)
[ ] Complete Phase 2 (Path Structure)
[ ] Reach 89% pass rate milestone
[ ] Decide on continuing vs shipping

Next Week:
[ ] Complete remaining phases
[ ] Achieve 100% pass rate
[ ] Update documentation
[ ] Merge to main

┌─────────────────────────────────────────────────────────────────────┐
│ RELATED DOCUMENTS                                                   │
└─────────────────────────────────────────────────────────────────────┘

📄 /docs/bdd-test-fix-plan.md      - Detailed implementation guide
📄 /docs/bdd-fix-summary.md        - Quick reference summary
📄 /docs/testing-tasks.md          - Original analysis and unit test fixes
📄 /docs/bdd-fix-roadmap.md        - This document (visual roadmap)
