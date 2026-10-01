"""Unit tests for SDD review-requirement and report validation."""
import importlib.util
from pathlib import Path
import unittest


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

    def test_report_path_must_be_owned_by_packet_change_surface(self) -> None:
        packet = self.packet.replace("`tasks/reviews/E18-T06.md`", "`tasks/other.md`")
        issues = check_tasks.review_record_issues(
            self.task_id, "self", packet, self.claim, self.handoff, self.report
        )
        self.assertIn(
            "E18-T06 packet Change Surface does not authorize tasks/reviews/E18-T06.md",
            issues,
        )

    def test_owns_substring_of_longer_path_does_not_authorize(self) -> None:
        packet = self.packet.replace(
            "`tasks/reviews/E18-T06.md`", "`tasks/reviews/E18-T06-extra.md`"
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", packet, self.claim, self.handoff, self.report
        )
        self.assertIn(
            "E18-T06 packet Change Surface does not authorize tasks/reviews/E18-T06.md",
            issues,
        )

    def test_owns_parenthesized_path_authorizes(self) -> None:
        packet = self.packet.replace(
            "`tasks/reviews/E18-T06.md`", "(tasks/reviews/E18-T06.md)"
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", packet, self.claim, self.handoff, self.report
        )
        self.assertEqual([], issues)

    def test_self_reviewer_must_match_task_claim_owner(self) -> None:
        report = self.report.replace(
            "**Reviewer:** author@example.test", "**Reviewer:** another@example.test"
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", self.packet, self.claim, self.handoff, report
        )
        self.assertIn(
            "E18-T06 self-reviewer must match the task claim owner", issues
        )

    def test_independent_reviewer_must_differ_from_author(self) -> None:
        packet = self.packet.replace(
            "Review Requirement:** self", "Review Requirement:** independent"
        ).replace("**Reviewer:** author@example.test", "**Reviewer:** reviewer@example.test")
        report = self.report.replace("Review Requirement:** self", "Review Requirement:** independent")
        issues = check_tasks.review_record_issues(
            self.task_id, "independent", packet, self.claim, self.handoff, report
        )
        self.assertIn(
            "E18-T06 independent reviewer must differ from packet author", issues
        )

        independent_report = report.replace(
            "**Reviewer:** author@example.test", "**Reviewer:** reviewer@example.test"
        )
        issues = check_tasks.review_record_issues(
            self.task_id,
            "independent",
            packet,
            self.claim,
            self.handoff,
            independent_report,
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

    def test_must_not_touch_only_does_not_authorize_report(self) -> None:
        packet = self.packet.replace("`tasks/reviews/E18-T06.md`", "`tasks/other.md`")
        packet = packet.replace(
            "**Owns:** `tasks/other.md`",
            "**Owns:** `tasks/other.md`\n**Must Not Touch:** `tasks/reviews/E18-T06.md`",
        )
        issues = check_tasks.review_record_issues(
            self.task_id, "self", packet, self.claim, self.handoff, self.report
        )
        self.assertIn(
            "E18-T06 packet Change Surface does not authorize tasks/reviews/E18-T06.md",
            issues,
        )

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


if __name__ == "__main__":
    unittest.main()
