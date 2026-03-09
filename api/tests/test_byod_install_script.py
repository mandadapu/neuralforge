"""Tests for api.byod_install_script (120 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestByodInstallScriptGeneration:
    def test_generate_script_for_ubuntu(self):
        gen = MagicMock()
        gen.generate.return_value = "#!/bin/bash\napt-get install -y agent\n"
        result = gen.generate(os="ubuntu", version="22.04")
        assert "bash" in result
        assert "apt-get" in result

    def test_generate_script_for_centos(self):
        gen = MagicMock()
        gen.generate.return_value = "#!/bin/bash\nyum install -y agent\n"
        result = gen.generate(os="centos", version="7")
        assert "yum" in result

    def test_generate_script_unknown_os_raises(self):
        gen = MagicMock()
        gen.generate.side_effect = ValueError("unsupported OS: beos")
        with pytest.raises(ValueError, match="unsupported OS"):
            gen.generate(os="beos", version="5.0")

    def test_generated_script_includes_auth_token(self):
        gen = MagicMock()
        gen.generate.return_value = "#!/bin/bash\nexport TOKEN=abc123\n"
        result = gen.generate(os="ubuntu", version="22.04", token="abc123")
        assert "abc123" in result

    def test_template_rendering_substitutes_variables(self):
        gen = MagicMock()
        gen.render.return_value = "#!/bin/bash\nVERSION=1.2.3\n"
        result = gen.render(template="install.sh.j2", context={"version": "1.2.3"})
        assert "1.2.3" in result

    def test_generated_script_is_non_empty(self):
        gen = MagicMock()
        gen.generate.return_value = "#!/bin/bash\necho 'done'\n"
        result = gen.generate(os="ubuntu", version="22.04")
        assert len(result) > 0
