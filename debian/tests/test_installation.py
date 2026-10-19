# This file is part of arduino-app-cli.
#
# SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
# SPDX-License-Identifier: GPL-3.0-or-later

"""Run with python3 -B debian/tests/test_installation.py -v; no root access needed."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


PACKAGE = Path(__file__).resolve().parents[1] / "arduino-app-cli"
MOCK_COMMAND = """#!PYTHON
import json
import os
from pathlib import Path
import sys

path = Path(os.environ["INSTALLATION_TEST_STATE"])
state = json.loads(path.read_text())
command = Path(sys.argv[0]).name
args = sys.argv[1:]
state["calls"].append([command, *args])
status = 0

if command == "getent":
    database, key = args
    if database == "passwd":
        account = state["accounts"].get(key)
        if account is None:
            account = next(
                (a for a in state["accounts"].values() if str(a["uid"]) == key),
                None,
            )
        if account is None:
            status = 2
        else:
            print("{name}:x:{uid}:100:Arduino:{home}:/bin/false".format(**account))
    elif key in state["groups"]:
        print(key + ":x:100:")
    else:
        status = 2
elif command == "adduser":
    if state.get("fail_adduser"):
        print("adduser: cannot allocate a UID", file=sys.stderr)
        status = 1
    else:
        if "--uid" in args:
            raise RuntimeError("The installer must delegate UID allocation to adduser")
        uid = 1000
        while any(a["uid"] == uid for a in state["accounts"].values()):
            uid += 1
        name = args[-1]
        state["accounts"][name] = {
            "name": name, "uid": uid, "home": state["arduino_home"]
        }
elif command in ("addgroup", "groupadd"):
    state["groups"].append(args[-1])
elif command == "chpasswd":
    state["password_input"] = sys.stdin.read()

path.write_text(json.dumps(state))
sys.exit(status)
"""


class InstallationTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.home = self.root / "arduino"
        self.busy_home = self.root / "busy"
        self.state_file = self.root / "state.json"
        self.bin = self.root / "bin"
        self.bin.mkdir()
        mock = self.bin / "mock"
        mock.write_text(MOCK_COMMAND.replace("#!PYTHON", "#!" + sys.executable, 1))
        mock.chmod(0o755)
        for command in (
            "getent", "adduser", "addgroup", "groupadd", "usermod",
            "systemctl", "arduino-app-cli", "chown", "chmod",
            "mkdir", "cp", "rm", "ln", "rmdir", "chpasswd",
        ):
            (self.bin / command).symlink_to(mock)
        self.environment = {
            **os.environ,
            "PATH": str(self.bin) + os.pathsep + os.environ["PATH"],
            "INSTALLATION_TEST_STATE": str(self.state_file),
        }
        self.save_state({
            "accounts": {},
            "groups": [],
            "calls": [],
            "arduino_home": str(self.home),
        })

    def save_state(self, state):
        self.state_file.write_text(json.dumps(state))

    def state(self):
        return json.loads(self.state_file.read_text())

    def add_account(self, name, uid, home):
        state = self.state()
        state["accounts"][name] = {"name": name, "uid": uid, "home": str(home)}
        self.save_state(state)

    def run_script(self, name, argument, helper=None, input=None):
        source = (PACKAGE / name).read_text()
        if helper:
            source = source.replace(
                "#LOAD_HELPER", (PACKAGE / "DEBIAN" / helper).read_text()
            )
        completion = self.root / "completion"
        completion.mkdir(exist_ok=True)
        source = source.replace("/etc/bash_completion.d/", str(completion) + "/")
        shell = "sh" if source.startswith("#!/bin/sh\n") else "bash"
        return subprocess.run(
            [shell, "-c", source, name, argument],
            input=input, text=True, capture_output=True, env=self.environment,
        )

    def test_creating_account_uses_first_available_uid(self):
        for occupied, expected in (
            ([], 1000), ([1000], 1001), ([1000, 1001], 1002), ([1000, 1002], 1001),
        ):
            with self.subTest(occupied=occupied):
                state = self.state()
                state["accounts"] = {}
                state["calls"] = []
                self.save_state(state)
                for uid in occupied:
                    self.add_account("busy" + str(uid), uid, self.busy_home)
                result = self.run_script("DEBIAN/preinst", "install")
                self.assertEqual(result.returncode, 0, result.stderr)
                state = self.state()
                self.assertEqual(state["accounts"]["arduino"]["uid"], expected)
                self.assertEqual(
                    [call for call in state["calls"] if call[0] == "usermod"],
                    [["usermod", "-aG", group, "arduino"]
                     for group in ("arduino", "sysupgrade", "docker")],
                )
                self.assertEqual(
                    [call for call in state["calls"] if call[0] == "adduser"],
                    [["adduser", "--ingroup", "arduino", "--home", "/home/arduino",
                      "--shell", "/bin/false", "--disabled-password", "arduino"]],
                )

    def test_existing_arduino_account_is_reused_on_install_and_upgrade(self):
        self.add_account("busy", 1000, self.busy_home)
        self.add_account("arduino", 1234, self.home)
        for argument in ("install", "upgrade"):
            result = self.run_script("DEBIAN/preinst", argument)
            self.assertEqual(result.returncode, 0, result.stderr)
        state = self.state()
        self.assertEqual(state["accounts"]["arduino"]["uid"], 1234)
        self.assertFalse(any(call[0] == "adduser" for call in state["calls"]))
        self.assertIn(["rm", "-rf", str(self.home / ".local/share/arduino-app-cli/examples")],
                      state["calls"])
        self.assertFalse(any(str(self.busy_home) in part
                             for call in state["calls"] for part in call))

    def test_account_creation_failure_stops_installation(self):
        state = self.state()
        state["fail_adduser"] = True
        self.save_state(state)
        result = self.run_script("DEBIAN/preinst", "install")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("cannot allocate a UID", result.stderr)
        self.assertFalse(any(call[0] == "usermod" for call in self.state()["calls"]))

    def test_postinstall_targets_arduino_account_and_home(self):
        self.add_account("busy", 1000, self.busy_home)
        self.add_account("arduino", 1234, self.home)
        result = self.run_script("DEBIAN/postinst", "configure", "postinst.helper.sh")
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.state()["calls"]
        self.assertIn(["chown", "-R", "arduino:arduino", "/var/lib/arduino-app-cli"], calls)
        self.assertIn(["rm", "-rf", str(self.home / ".local/share/arduino-app-cli")], calls)
        for profile in (".claude/CLAUDE.md", ".gemini/GEMINI.md",
                        ".codex/AGENTS.md", ".copilot/copilot-instructions.md"):
            self.assertIn(["chown", "-h", "arduino:arduino", str(self.home / profile)], calls)
        self.assertFalse(any(str(self.busy_home) in part for call in calls for part in call))

    def test_purge_targets_arduino_home(self):
        self.add_account("busy", 1000, self.busy_home)
        self.add_account("arduino", 1234, self.home)
        for home in (self.home, self.busy_home):
            (home / ".claude").mkdir(parents=True)
            (home / ".claude/CLAUDE.md").symlink_to("/etc/arduino-app-cli/AGENTS.md")
        result = self.run_script("DEBIAN/postrm", "purge", "postrm.helper.sh")
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.state()["calls"]
        self.assertIn(["rm", "-f", str(self.home / ".claude/CLAUDE.md")], calls)
        self.assertFalse(any(str(self.busy_home) in part for call in calls for part in call))

    def test_password_helper_targets_arduino(self):
        result = self.run_script("usr/local/bin/arduino-passwd", "", input="test-password\n")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.state()["password_input"], "arduino:test-password\n")

    def test_empty_password_is_rejected(self):
        result = self.run_script("usr/local/bin/arduino-passwd", "", input="\n")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(call[0] == "chpasswd" for call in self.state()["calls"]))

    def test_service_and_sudo_rules_use_username(self):
        service = (PACKAGE / "etc/systemd/system/arduino-app-cli.service").read_text()
        self.assertIn("\nUser=arduino\n", service)
        for rule in ("10-arduino-passwd", "20-arduino-ssh", "30-arduino-keyboard"):
            source = (PACKAGE / "etc/sudoers.d" / rule).read_text()
            self.assertTrue(source.startswith("arduino ALL="))


if __name__ == "__main__":
    unittest.main()
