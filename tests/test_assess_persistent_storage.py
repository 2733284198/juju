"""Tests for assess_persistent_storage module."""

import logging
import StringIO
from textwrap import dedent

from mock import (
    Mock,
    patch,
    )

import assess_persistent_storage as aps
from assess_persistent_storage import (
    assess_persistent_storage,
    parse_args,
    main,
    )
from jujupy import fake_juju_client
from tests import (
    parse_error,
    TestCase,
    )


class TestParseArgs(TestCase):

    def test_common_args(self):
        args = parse_args(["an-env", "/bin/juju", "/tmp/logs", "an-env-mod"])
        self.assertEqual("an-env", args.env)
        self.assertEqual("/bin/juju", args.juju_bin)
        self.assertEqual("/tmp/logs", args.logs)
        self.assertEqual("an-env-mod", args.temp_env_name)
        self.assertEqual(False, args.debug)

    def test_help(self):
        fake_stdout = StringIO.StringIO()
        with parse_error(self) as fake_stderr:
            with patch("sys.stdout", fake_stdout):
                parse_args(["--help"])
        self.assertEqual("", fake_stderr.getvalue())
        self.assertNotIn("TODO", fake_stdout.getvalue())


class TestGetStorageSystems(TestCase):

    def test_returns_single_known_filesystem(self):
        storage_json = dedent("""\
        filesystems:
            0/0:
                provider-id: 0/1
                storage: single-fs/1
                attachments:
                machines:
                    "0":
                    mount-point: /srv/single-fs
                    read-only: false
                    life: alive
                units:
                    dummy-storage/0:
                    machine: "0"
                    location: /srv/single-fs
                    life: alive
                pool: rootfs
                size: 28775
                life: alive
                status:
                current: attached
                since: 14 Mar 2017 17:01:15+13:00
        """)
        client = Mock()
        client.list_storage.return_value = storage_json

        self.assertEqual(
            aps.get_storage_filesystems(client, 'single-fs'),
            ['single-fs/1'])

    def test_returns_empty_list_when_none_found(self):
        storage_json = dedent("""\
        filesystems:
            0/0:
                provider-id: 0/1
                storage: single-fs/1
                attachments:
                machines:
                    "0":
                    mount-point: /srv/single-fs
                    read-only: false
                    life: alive
                units:
                    dummy-storage/0:
                    machine: "0"
                    location: /srv/single-fs
                    life: alive
                pool: rootfs
                size: 28775
                life: alive
                status:
                current: attached
                since: 14 Mar 2017 17:01:15+13:00
        """)
        client = Mock()
        client.list_storage.return_value = storage_json

        self.assertEqual(
            aps.get_storage_filesystems(client, 'not-found'),
            [])

    def test_returns_many_for_multiple_finds(self):
        storage_json = dedent("""\
        filesystems:
            "0/0":
                provider-id: 0/1
                storage: multi-fs/1
                attachments:
                machines:
                    "0":
                    mount-point: /srv/multi-fs
                    read-only: false
                    life: alive
                units:
                    dummy-storage/0:
                    machine: "0"
                    location: /srv/multi-fs
                    life: alive
                pool: rootfs
                size: 28775
                life: alive
                status:
                current: attached
                since: 14 Mar 2017 17:01:15+13:00
            0/1:
                provider-id: 0/2
                storage: multi-fs/2
                attachments:
                machines:
                    "0":
                    mount-point: /srv/multi-fs
                    read-only: false
                    life: alive
                units:
                    dummy-storage/0:
                    machine: "0"
                    location: /srv/multi-fs
                    life: alive
                pool: rootfs
                size: 28775
                life: alive
                status:
                current: attached
                since: 14 Mar 2017 17:01:15+13:00
        """)
        client = Mock()
        client.list_storage.return_value = storage_json

        self.assertEqual(
            aps.get_storage_filesystems(client, 'multi-fs'),
            ['multi-fs/1', 'multi-fs/2'])