"""JUnit XML generation for Spyglass."""

from __future__ import annotations

import os
import xml.etree.ElementTree as ET
from xml.dom import minidom


def write_junit(
    artifact_dir: str,
    eval_name: str,
    per_case: dict[str, dict[str, dict]],
) -> None:
    """Write junit_{eval_name}.xml from harness per_case judge results."""
    total_tests = 0
    total_failures = 0
    total_errors = 0
    cases: list[ET.Element] = []

    for case_name in sorted(per_case.keys()):
        case_results = per_case[case_name]
        for judge_name in sorted(case_results.keys()):
            result = case_results[judge_name]
            total_tests += 1
            tc = ET.Element("testcase")
            tc.set("name", f"[{eval_name}] {case_name} {judge_name}")
            tc.set("classname", f"{eval_name}.{case_name}")

            error = result.get("error")
            value = result.get("value")
            rationale = result.get("rationale", "")

            if error:
                err_el = ET.SubElement(tc, "error")
                err_el.set("message", str(error))
                err_el.text = str(error)
                total_errors += 1
            elif value is False:
                msg = rationale or f"{judge_name} check did not pass."
                fail_el = ET.SubElement(tc, "failure")
                fail_el.set("message", msg)
                fail_el.text = msg
                total_failures += 1
            elif value is None and rationale and "Skipped" in rationale:
                pass
            elif value is None:
                msg = rationale or "judge returned no value"
                err_el = ET.SubElement(tc, "error")
                err_el.set("message", msg)
                err_el.text = msg
                total_errors += 1

            cases.append(tc)

    suite = ET.Element("testsuite")
    suite.set("name", eval_name)
    suite.set("tests", str(total_tests))
    suite.set("failures", str(total_failures))
    suite.set("errors", str(total_errors))
    for tc in cases:
        suite.append(tc)

    suites = ET.Element("testsuites")
    suites.append(suite)

    rough = ET.tostring(suites, encoding="unicode")
    parsed = minidom.parseString(rough)
    pretty = parsed.toprettyxml(indent="  ")
    lines = [line for line in pretty.split("\n") if line.strip()]
    xml_body = "\n".join(lines[1:]) if lines else pretty

    path = os.path.join(artifact_dir, f"junit_{eval_name}.xml")
    with open(path, "w") as f:
        f.write("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
        f.write(xml_body)
        if not xml_body.endswith("\n"):
            f.write("\n")
    os.chmod(path, 0o600)
