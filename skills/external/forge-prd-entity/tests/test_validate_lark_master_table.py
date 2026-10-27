#!/usr/bin/env python3

import importlib.util
import json
import subprocess
import sys
import unittest
from pathlib import Path


SCRIPT = Path(__file__).parents[1] / "scripts" / "validate_lark_master_table.py"
SPEC = importlib.util.spec_from_file_location("validate_lark_master_table", SCRIPT)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


def table(rows: str) -> str:
    return '<table><thead><tr><th>功能</th><th>原型图与功能说明</th></tr></thead><tbody>' + rows + '</tbody></table>'


def row(name: str, hierarchy: str = "批量导入 / 任务管理", *, image: bool = True) -> str:
    visual = '<p>原型图</p><img src="actual-token"/>' if image else ''
    return (
        f'<tr><td><h3>{hierarchy}</h3><h4 align="center">{name}</h4></td>'
        f'<td>{visual}<p>功能与效果 查看导入任务并按状态取消或重试失败项。</p></td></tr>'
    )


VALID = '<fragment>' + table(row('导入任务列表')) + '</fragment>'
GROUPS = '<fragment>' + (
    '<h2>3.1 任务管理</h2>' + table(row('导入任务列表') + row('失败通知', image=False))
    + '<h2>3.2 结果管理</h2>' + table(row('导入任务详情', '批量导入 / 结果管理'))
    + '<h2>3.3 系统配置</h2>' + table(row('导入模板', '批量导入 / 系统配置'))
) + '</fragment>'


class MasterTableValidatorTests(unittest.TestCase):
    def test_explicit_single_table_still_supported(self):
        result = MODULE.validate_xml(VALID, expected_modules=1, expected_tables=1)
        self.assertTrue(result['ok'], result)

    def test_architecture_grouped_tables_in_requested_order(self):
        result = MODULE.validate_xml(
            GROUPS, expected_modules=4, expected_tables=3,
            expected_sections=['任务管理', '结果管理', '系统配置'], hierarchical_labels=True,
            visual_modules=['导入任务列表', '导入任务详情', '导入模板'],
        )
        self.assertTrue(result['ok'], result)
        self.assertEqual(result['table_count'], 3)
        self.assertEqual(result['row_count'], 4)

    def test_no_universal_table_count(self):
        self.assertTrue(MODULE.validate_xml(GROUPS)['ok'])

    def test_explicit_single_table_rejects_split_tables(self):
        result = MODULE.validate_xml(GROUPS, expected_tables=1)
        self.assertFalse(result['ok'])
        self.assertEqual(result['master_table_count'], 3)

    def test_wrong_architecture_order_fails(self):
        result = MODULE.validate_xml(GROUPS, expected_sections=['任务管理', '系统配置', '结果管理'])
        self.assertFalse(result['ok'])
        self.assertTrue(any('section order' in error for error in result['errors']))

    def test_missing_branch_fails(self):
        two = '<fragment><h2>任务管理</h2>' + table(row('导入任务列表'))
        two += '<h2>系统配置</h2>' + table(row('导入模板')) + '</fragment>'
        result = MODULE.validate_xml(two, expected_tables=3, expected_sections=['任务管理', '结果管理', '系统配置'])
        self.assertFalse(result['ok'])

    def test_optional_fields_do_not_require_goals_components_interactions(self):
        result = MODULE.validate_xml(VALID)
        self.assertTrue(result['ok'], result)
        self.assertEqual(result['rows'][0]['present_labels'], ['功能与效果'])

    def test_explicit_required_fields_are_checked(self):
        result = MODULE.validate_xml(VALID, required_labels=['交互逻辑'])
        self.assertFalse(result['ok'])
        self.assertTrue(any('missing agreed field' in error for error in result['errors']))

    def test_text_only_automatic_capability_is_allowed(self):
        xml = table(row('失败通知', image=False))
        self.assertTrue(MODULE.validate_xml(xml)['ok'])
        self.assertFalse(MODULE.validate_xml(xml, require_visuals=True)['ok'])

    def test_required_ui_visual_is_missing(self):
        xml = table(row('导入任务详情', image=False))
        result = MODULE.validate_xml(xml, visual_modules=['导入任务详情'])
        self.assertFalse(result['ok'])

    def test_misspelled_visual_module_is_not_silently_ignored(self):
        self.assertFalse(MODULE.validate_xml(VALID, visual_modules=['不存在的页面'])['ok'])

    def test_image_after_explanation_fails(self):
        wrong = VALID.replace(
            '<p>原型图</p><img src="actual-token"/><p>功能与效果 查看导入任务并按状态取消或重试失败项。</p>',
            '<p>功能与效果 查看导入任务并按状态取消或重试失败项。</p><img src="actual-token"/>',
        )
        self.assertFalse(MODULE.validate_xml(wrong)['ok'])

    def test_label_only_filler_fails(self):
        wrong = VALID.replace('功能与效果 查看导入任务并按状态取消或重试失败项。', '页面目标')
        result = MODULE.validate_xml(wrong)
        self.assertFalse(result['ok'])
        self.assertTrue(any('substantive explanation' in error for error in result['errors']))

    def test_unresolved_image_source_fails(self):
        self.assertFalse(MODULE.validate_xml(VALID.replace('actual-token', '[图片token]'))['ok'])
        self.assertFalse(MODULE.validate_xml(VALID.replace('actual-token', ''))['ok'])

    def test_combined_label_only_filler_fails(self):
        wrong = VALID.replace('功能与效果 查看导入任务并按状态取消或重试失败项。', '功能与效果 / 页面目标 / 核心组件 / 交互逻辑')
        self.assertFalse(MODULE.validate_xml(wrong)['ok'])

    def test_brief_meaningful_rule_does_not_require_padding(self):
        brief = VALID.replace('功能与效果 查看导入任务并按状态取消或重试失败项。', '状态与异常 保留旧版。')
        self.assertTrue(MODULE.validate_xml(brief)['ok'])

    def test_hierarchy_is_h3_at_top_and_title_h4(self):
        self.assertTrue(MODULE.validate_xml(VALID, hierarchical_labels=True)['ok'])
        wrong = VALID.replace('<h3>批量导入 / 任务管理</h3>', '<p>批量导入 / 任务管理</p>')
        self.assertFalse(MODULE.validate_xml(wrong, hierarchical_labels=True)['ok'])
        wrong = VALID.replace('<h4 align="center">导入任务列表</h4>', '<h3>导入任务列表</h3>')
        self.assertFalse(MODULE.validate_xml(wrong, hierarchical_labels=True)['ok'])

    def test_image_in_left_cell_fails(self):
        wrong = VALID.replace('<h3>批量导入 / 任务管理</h3>', '<img src="wrong"/><h3>批量导入 / 任务管理</h3>')
        self.assertFalse(MODULE.validate_xml(wrong)['ok'])

    def test_duplicate_full_feature_identity_fails(self):
        self.assertFalse(MODULE.validate_xml(table(row('导入任务列表') + row('导入任务列表')))['ok'])

    def test_same_title_with_distinct_mode_ownership_is_allowed(self):
        xml = table(row('导入任务', '批量导入 / 客户导入') + row('导入任务', '批量导入 / 线索导入'))
        self.assertTrue(MODULE.validate_xml(xml)['ok'])

    def test_body_header_and_fragment_input_supported(self):
        xml = '<table><tbody><tr><th>模块</th><th>原型图与说明</th></tr>' + row('导入任务列表') + '</tbody></table>'
        xml += '<p>其他说明</p>'
        result = MODULE.validate_xml(xml, expected_modules=1)
        self.assertTrue(result['ok'], result)

    def test_empty_table_fails(self):
        self.assertFalse(MODULE.validate_xml(table(''))['ok'])

    def test_cli_accepts_contract_and_reports_structure_not_quality(self):
        completed = subprocess.run(
            [sys.executable, str(SCRIPT), '-', '--expected-tables', '3', '--expected-section', '任务管理',
             '--expected-section', '结果管理', '--expected-section', '系统配置', '--hierarchical-labels',
             '--visual-module', '导入任务详情'],
            input=GROUPS, capture_output=True, text=True, encoding='utf-8', check=False,
        )
        self.assertEqual(completed.returncode, 0, completed.stderr or completed.stdout)
        self.assertTrue(json.loads(completed.stdout)['ok'])
        self.assertNotIn('score', completed.stdout.lower())

    def test_cli_reports_invalid_xml(self):
        completed = subprocess.run(
            [sys.executable, str(SCRIPT), '-'], input='<fragment><table>',
            capture_output=True, text=True, check=False,
        )
        self.assertEqual(completed.returncode, 2)
        self.assertIn('ERROR:', completed.stderr)


if __name__ == '__main__':
    unittest.main()
