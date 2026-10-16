"""Unit tests for SDD review-requirement and report validation."""
from contextlib import redirect_stdout
import importlib.util
from io import StringIO
from pathlib import Path
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).with_name("check-tasks.py")
SPEC = importlib.util.spec_from_file_location("check_tasks", SCRIPT)
check_tasks = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(check_tasks)


class ReviewRecordValidationTest(unittest.TestCase):
    def setUp(self) -> None:
        self.task_id = "E18-T06"
        self.packet = """**Task:** E18-T06
**SDD Packet Version:** 2
**Author:** author@example.test
**Reviewer:** author@example.test
**Review Requirement:** self
**Notes:** ordinary task

## Change Surface
**Owns:** `tasks/reviews/E18-T06.md`
"""
        self.claim = (
            "**Owner:** author@example.test\n"
            "**Base Commit:** " + "a" * 40 + "\n"
        )
        self.handoff = (
            "**Base / Head:** " + "a" * 40 + " / " + "b" * 40 + "\n"
            "Review: `tasks/reviews/E18-T06.md`\n"
        )
        self.report = """**Task:** E18-T06
**Epic:** E18
**Base Commit:** aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
**Reviewed Commit:** bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
**Reviewer:** author@example.test
**Review Date:** 2026-09-28T04:48:35Z
**Review Requirement:** self
**Verdict:** PASS
**Risk Score:** Low

## Summary
Matches the plan.
## Findings
None.
## Verification Performed
`python3 tasks/scripts/test_check_tasks.py` — pass.
## Test Coverage Gaps
None.
## SDD / Docs Gaps
None.
## Non-blocking Observations
None.
## Final Review Status
PASS
## Not Verified
None.
## Resolution Notes
None yet.
"""

    def test_valid_self_review_passes(self) -> None:
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, self.handoff, self.report
        )
        self.assertEqual([], issues)

    def test_legacy_packet_without_review_field_remains_compatible(self) -> None:
        mode, issues = check_tasks.packet_review_issues(
            "**Task:** E01-T01\n**Author:** old-author\n"
        )
        self.assertIsNone(mode)
        self.assertEqual([], issues)

    def test_version_two_packet_requires_review_field(self) -> None:
        mode, issues = check_tasks.packet_review_issues(
            "**SDD Packet Version:** 2\n**Task:** E18-T06\n"
        )
        self.assertIsNone(mode)
        self.assertIn("version 2 packet has no Review Requirement", issues)

    def test_version_three_packet_does_not_require_reviewer_identity_or_harness(self) -> None:
        mode, issues = check_tasks.packet_review_issues(
            "**SDD Packet Version:** 3\n"
            "**Review Requirement:** independent\n"
        )
        self.assertEqual("independent", mode)
        self.assertEqual([], issues)

    def test_none_mode_requires_rationale(self) -> None:
        _, issues = check_tasks.packet_review_issues(
            "**SDD Packet Version:** 2\n**Review Requirement:** none\n"
            "**Notes:** None\n"
        )
        self.assertIn("Review Requirement 'none' needs a rationale in Notes", issues)

    def test_missing_report_and_handoff_link_are_reported(self) -> None:
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, "", None
        )
        self.assertEqual(1, len(issues))
        self.assertIn("has no tasks/reviews/E18-T06.md", issues[0])

        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim,
            "Base / Head: " + "a" * 40 + " / " + "b" * 40 + "\nno report link\n",
            self.report,
        )
        self.assertIn(
            "E18-T06 handoff does not link tasks/reviews/E18-T06.md", issues
        )

    def test_report_path_need_not_be_owned_by_packet_change_surface(self) -> None:
        packet = self.packet.replace("`tasks/reviews/E18-T06.md`", "`tasks/other.md`")
        issues = check_tasks.review_record_issues(
            self.task_id, "self", packet, self.claim, self.handoff, self.report
        )
        self.assertEqual([], issues)

    def test_owns_substring_is_not_used_as_a_review_write_gate(self) -> None:
        packet = self.packet.replace(
            "`tasks/reviews/E18-T06.md`", "`tasks/reviews/E18-T06-extra.md`"
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", packet, self.claim, self.handoff, self.report
        )
        self.assertEqual([], issues)

    def test_report_path_is_not_authorized_by_packet_owns(self) -> None:
        packet = self.packet.replace(
            "`tasks/reviews/E18-T06.md`", "(tasks/other.md)"
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", packet, self.claim, self.handoff, self.report
        )
        self.assertEqual([], issues)

    def test_self_review_accepts_reviewer_identity_and_harness_from_any_agent(self) -> None:
        report = self.report.replace(
            "**Reviewer:** author@example.test", "**Reviewer:** another@example.test"
        ).replace("**Reviewer Harness:** tool and agent/model identity",
                  "**Reviewer Harness:** Different Harness / Other Agent")
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, self.handoff, report
        )
        self.assertEqual([], issues)

    def test_independent_review_accepts_unspecified_or_matching_reviewer_metadata(self) -> None:
        packet = self.packet.replace(
            "Review Requirement:** self", "Review Requirement:** independent"
        ).replace("**Reviewer:** author@example.test", "**Reviewer:** packet-suggestion")
        report = self.report.replace("Review Requirement:** self", "Review Requirement:** independent")
        issues = check_tasks.review_record_issues(
            self.task_id, "independent", packet, self.claim, self.handoff, report
        )
        self.assertEqual([], issues)

    def test_malformed_commit_and_verdict_are_rejected(self) -> None:
        report = self.report.replace("a" * 40, "not-a-sha").replace(
            "**Verdict:** PASS", "**Verdict:** MAYBE"
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, self.handoff, report
        )
        self.assertIn("E18-T06 review report has invalid Base Commit SHA", issues)
        self.assertIn("E18-T06 review report has unsupported Verdict", issues)

    def test_invalid_calendar_review_date_is_rejected(self) -> None:
        report = self.report.replace(
            "2026-09-28T04:48:35Z", "2026-99-99T99:99:99Z"
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, self.handoff, report
        )
        self.assertIn("E18-T06 review report has invalid RFC3339 Review Date", issues)

    def test_unresolved_review_cannot_be_accepted_as_completed(self) -> None:
        report = self.report.replace(
            "**Verdict:** PASS", "**Verdict:** REVISION REQUIRED"
        )
        report = report.replace(
            "## Final Review Status\nPASS", "## Final Review Status\nREVISION REQUIRED"
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, self.handoff, report
        )
        self.assertIn(
            "E18-T06 review Verdict 'REVISION REQUIRED' does not permit completion",
            issues,
        )

    def test_final_status_must_match_verdict_metadata(self) -> None:
        report = self.report.replace(
            "## Final Review Status\nPASS", "## Final Review Status\nBLOCKED"
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, self.handoff, report
        )
        self.assertIn(
            "E18-T06 Final Review Status must contain only its Verdict", issues
        )

    def test_pass_final_status_cannot_override_non_permitting_verdict(self) -> None:
        report = self.report.replace(
            "**Verdict:** PASS", "**Verdict:** REVISION REQUIRED"
        )
        report = report.replace(
            "## Final Review Status\nPASS", "## Final Review Status\nPASS"
        )
        report = report.replace(
            "## Resolution Notes\nNone yet.",
            "## Resolution Notes\n**2026-09-28T06:40:00Z — fixed CR-001, CR-002.**",
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, self.handoff, report
        )
        self.assertIn(
            "E18-T06 Final Review Status must contain only its Verdict", issues
        )
        self.assertIn(
            "E18-T06 review Verdict 'REVISION REQUIRED' does not permit completion",
            issues,
        )
        self.assertIn(
            "Final Review Status PASS does not override a non-permitting Verdict",
            " | ".join(issues),
        )

    def test_pass_with_minor_issues_is_not_misread_as_pass(self) -> None:
        report = self.report.replace(
            "**Verdict:** PASS", "**Verdict:** PASS WITH MINOR ISSUES"
        )
        report = report.replace(
            "## Final Review Status\nPASS",
            "## Final Review Status\n- **PASS WITH MINOR ISSUES** — only low findings remain",
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, self.handoff, report
        )
        self.assertEqual([], issues)

    def test_must_not_touch_does_not_gate_review_report_validation(self) -> None:
        packet = self.packet.replace("`tasks/reviews/E18-T06.md`", "`tasks/other.md`")
        packet = packet.replace(
            "**Owns:** `tasks/other.md`",
            "**Owns:** `tasks/other.md`\n**Must Not Touch:** `tasks/reviews/E18-T06.md`",
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", packet, self.claim, self.handoff, self.report
        )
        self.assertEqual([], issues)

    def test_report_review_requirement_is_case_insensitive(self) -> None:
        report = self.report.replace(
            "**Review Requirement:** self", "**Review Requirement:** Self"
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, self.handoff, report
        )
        self.assertEqual([], issues)

    def test_reviewed_commit_must_appear_in_handoff(self) -> None:
        handoff = "Review: `tasks/reviews/E18-T06.md`\n"
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, handoff, self.report
        )
        self.assertIn("E18-T06 Reviewed Commit not in handoff Base / Head line", issues)

    def test_reviewed_commit_in_prose_does_not_satisfy_binding(self) -> None:
        handoff = (
            "A reviewer once committed " + "b" * 40 + " in unrelated prose.\n"
            "Review: `tasks/reviews/E18-T06.md`\n"
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, handoff, self.report
        )
        self.assertIn("E18-T06 Reviewed Commit not in handoff Base / Head line", issues)

    def test_base_commit_must_match_claim(self) -> None:
        report = self.report.replace("a" * 40, "c" * 40)
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, self.handoff, report
        )
        self.assertIn(
            "E18-T06 review Base Commit does not match claim Base Commit", issues
        )


class ReviewWritePolicyValidationTest(unittest.TestCase):
    def setUp(self) -> None:
        self.task_id = "E18-T06"
        self.packet = """**Task:** E18-T06
**SDD Packet Version:** 3
**Spec Status:** ready
**Decision:** approved
**Author:** implementer
**Reviewer:** suggested-reviewer
**Reviewer Harness:** suggested-harness
**Review Requirement:** independent

## Change Surface
**Owns:** `tasks/other.md`
"""
        self.claim = f"""**Task:** {self.task_id}
**Status:** released
**Owner:** implementer
**Harness:** OpenCode
**Base Commit:** {'a' * 40}
**Started At:** 2026-10-14T01:00:00Z
**Lease Until:** 2026-10-15T02:00:00Z
**Released At:** 2026-10-14T01:45:00Z
"""
        self.handoff = (
            "**Base / Head:** " + "a" * 40 + " / " + "b" * 40 + "\n"
            "Review: `tasks/reviews/E18-T06.md`\n"
        )
        self.report = self._report()

    def test_v3_packet_does_not_require_reviewer_identity_or_harness(self) -> None:
        packet = self.packet.replace(
            "**Reviewer:** suggested-reviewer\n", ""
        ).replace("**Reviewer Harness:** suggested-harness\n", "")
        mode, issues = check_tasks.packet_review_issues(packet)
        self.assertEqual("independent", mode)
        self.assertEqual([], issues)

    def test_arbitrary_reviewer_and_harness_pass_with_released_claim_and_unowned_path(self) -> None:
        report = self.report.replace(
            "**Reviewer:** reviewer-one", "**Reviewer:** another-agent"
        ).replace("**Reviewer Harness:** Tool A", "**Reviewer Harness:** Different Tool B")
        issues = check_tasks.review_record_issues(
            self.task_id,
            "independent",
            self.packet,
            self.claim,
            self.handoff,
            report,
        )
        self.assertEqual([], issues)

    def test_absent_claim_does_not_gate_review_content(self) -> None:
        issues = check_tasks.review_record_issues(
            self.task_id,
            "independent",
            self.packet,
            "",
            self.handoff,
            self.report,
        )
        self.assertEqual([], issues)

    def test_expired_claim_does_not_gate_review_content(self) -> None:
        expired_claim = self.claim.replace(
            "**Status:** released", "**Status:** active"
        ).replace(
            "**Lease Until:** 2026-10-15T02:00:00Z",
            "**Lease Until:** 2026-10-14T01:59:00Z",
        ).replace("**Released At:** 2026-10-14T01:45:00Z\n", "")
        issues = check_tasks.review_record_issues(
            self.task_id,
            "independent",
            self.packet,
            expired_claim,
            self.handoff,
            self.report,
        )
        self.assertEqual([], issues)

    def test_multiple_attributed_reviewer_contributions_are_valid(self) -> None:
        report = self.report.replace(
            "## Findings\nNone.",
            "## Findings\n"
            "### CR-001 — First review contribution [Low] [Confirmed]\n"
            "- **Reviewer:** reviewer-one (Tool A)\n\n"
            "### CR-002 — Second review contribution [Low] [Risk]\n"
            "- **Reviewer:** reviewer-two (Tool B)",
        ).replace(
            "## Resolution Notes\nNone yet.",
            "## Resolution Notes\n"
            "### Contribution — reviewer-one\n"
            "### Contribution — reviewer-two",
        )
        issues = check_tasks.review_record_issues(
            self.task_id,
            "independent",
            self.packet,
            self.claim,
            self.handoff,
            report,
        )
        self.assertEqual([], issues)

    def test_review_preflight_cli_entrypoint_is_removed(self) -> None:
        source = SCRIPT.read_text()
        self.assertNotIn("--review-preflight", source)
        self.assertNotIn("def review_preflight_main", source)
        self.assertNotIn("def review_preflight_issues", source)

    def test_old_review_preflight_option_is_rejected(self) -> None:
        output = StringIO()
        with patch.object(
            check_tasks.sys,
            "argv",
            [str(SCRIPT), "--review-preflight"],
        ), redirect_stdout(output):
            exit_status = check_tasks.main()
        self.assertEqual(2, exit_status)
        self.assertIn("unknown option(s): --review-preflight", output.getvalue())

    def test_reviewer_markdown_writes_ignore_legacy_claim_gates(self) -> None:
        packet_template = (check_tasks.SPECS / "_TEMPLATE.md").read_text()
        claim_template = (check_tasks.CLAIMS / "_TEMPLATE.md").read_text()
        opencode_reviewer = (
            check_tasks.BASE / ".opencode/agents/reviewer.md"
        ).read_text()
        antigravity_reviewer = (
            check_tasks.BASE / ".agents/agents/reviewer.md"
        ).read_text()
        shared_policy = (check_tasks.BASE / "AGENTS.md").read_text()
        sdd_policy = (check_tasks.BASE / "tasks/SDD.md").read_text()
        readme = (check_tasks.BASE / "README.md").read_text()

        self.assertIn("**Reviewer:** optional suggestion", packet_template)
        self.assertIn("**Reviewer Harness:** optional suggestion", packet_template)
        self.assertNotIn("Review Authorizations", claim_template)
        self.assertIn('resource: "*"\n    effect: deny', opencode_reviewer)
        self.assertIn('resource: "*.md"\n    effect: allow', opencode_reviewer)
        self.assertIn("Markdown files anywhere in", opencode_reviewer)
        self.assertIn("Markdown files anywhere in", antigravity_reviewer)
        reviewer_description = (
            "Markdown writes are not claim-gated, and non-Markdown edits are prohibited."
        )
        reviewer_canary = "[WORKSPACE-REVIEWER-CANARY: no-claim-gate-v1]"
        reviewer_canary_instructions = """## Workspace reviewer profile canary

On your first response in each review conversation, begin with this exact
standalone line:

`[WORKSPACE-REVIEWER-CANARY: no-claim-gate-v1]`

This identifies the workspace reviewer prompt when the agent picker hides
descriptions. It is diagnostic only: never wait for confirmation or use the
canary as an authorization, review, file-tool, or persistence gate. Continue the
requested review immediately; omit the canary from later responses."""
        self.assertIn(reviewer_description, opencode_reviewer)
        self.assertIn(reviewer_description, antigravity_reviewer)
        for reviewer in (opencode_reviewer, antigravity_reviewer):
            self.assertIn(reviewer_canary, reviewer)
            self.assertIn(reviewer_canary_instructions, reviewer)
            self.assertIn(
                "On your first response in each review conversation",
                reviewer,
            )
            self.assertIn("diagnostic only", reviewer)
            self.assertIn("supersedes any legacy checklist", reviewer)
            self.assertIn("invoke the available file tool immediately", reviewer)
            self.assertNotIn("persistence checklist", reviewer.lower())
            self.assertIn("Only an actual file-tool or platform", reviewer)
            self.assertLess(
                reviewer.index(reviewer_canary),
                reviewer.index("You are a Principal Go Engineer"),
            )
        self.assertIn("This global rule supersedes legacy", shared_policy)
        self.assertIn(
            "This global write rule applies across task-packet versions",
            sdd_policy,
        )
        self.assertIn("start a fresh review conversation", readme)
        self.assertIn(reviewer_canary, readme)
        self.assertIn("The canary is diagnostic only", readme)
        self.assertIn("non-Markdown", antigravity_reviewer)

    def _report(self) -> str:
        return f"""**Task:** {self.task_id}
**Epic:** E18
**Base Commit:** {'a' * 40}
**Reviewed Commit:** {'b' * 40}
**Reviewer:** reviewer-one
**Reviewer Harness:** Tool A
**Review Date:** 2026-10-14T02:00:00Z
**Review Requirement:** independent
**Verdict:** PASS
**Risk Score:** Low

## Summary
Matches the plan.
## Findings
None.
## Verification Performed
`python3 tasks/scripts/test_check_tasks.py` — pass.
## Test Coverage Gaps
None.
## SDD / Docs Gaps
None.
## Non-blocking Observations
None.
## Final Review Status
PASS
## Not Verified
None.
## Resolution Notes
None yet.
"""


if __name__ == "__main__":
    unittest.main()
