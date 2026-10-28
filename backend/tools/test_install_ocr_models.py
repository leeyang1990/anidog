import hashlib,os,pathlib,shutil,subprocess,tempfile,unittest

class ModelInstallation(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
  self.root=pathlib.Path(self.tmp.name);self.source=self.root/'source';self.source.mkdir();self.target=self.root/'models with spaces';self.target.mkdir();self.bin=self.root/'bin';self.bin.mkdir()
  shutil.copyfile(pathlib.Path(__file__).with_name('install-ocr-models.sh'),self.source/'install.sh')
  self.payload=b'fixture model, not a real language model';self.sha=hashlib.sha256(self.payload).hexdigest()
  (self.source/'ocr-models.tsv').write_text(f'chi_sim {self.sha} https://fixture.invalid/model\n')
  (self.bin/'wget').write_text('#!/bin/sh\nprintf x >> "$OCR_TEST_CALLS"\n[ "$OCR_TEST_FAIL" = no ] || exit 1\nwhile [ "$1" != -O ]; do shift; done\nprintf "fixture model, not a real language model" > "$2"\n')
  (self.bin/'timeout').write_text('#!/bin/sh\n[ "$1" = 45 ] || exit 4\nshift\nexec "$@"\n')
  for p in self.bin.iterdir():p.chmod(0o700)
  self.calls=self.root/'calls';self.env=dict(os.environ,PATH=str(self.bin)+os.pathsep+os.environ['PATH'],OCR_TEST_CALLS=str(self.calls),OCR_TEST_FAIL='no')
 def run_installer(self):return subprocess.run(['sh',str(self.source/'install.sh'),str(self.target)],env=self.env,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
 def test_install_and_offline_idempotence(self):
  self.assertEqual(self.run_installer().returncode,0);self.assertEqual((self.target/'chi_sim.traineddata').read_bytes(),self.payload)
  self.env['OCR_TEST_FAIL']='yes';self.assertEqual(self.run_installer().returncode,0);self.assertEqual(self.calls.read_text(),'x')
 def test_failure_keeps_previous_model_and_bounds_retries(self):
  dest=self.target/'chi_sim.traineddata';dest.write_bytes(b'previous model');self.env['OCR_TEST_FAIL']='yes'
  self.assertNotEqual(self.run_installer().returncode,0);self.assertEqual(dest.read_bytes(),b'previous model');self.assertEqual(self.calls.read_text(),'xxx');self.assertEqual(list(self.target.iterdir()),[dest])
 def test_untrusted_bytes_do_not_replace_model(self):
  (self.source/'ocr-models.tsv').write_text('chi_sim '+'0'*64+' https://fixture.invalid/model\n');dest=self.target/'chi_sim.traineddata';dest.write_bytes(b'previous model')
  self.assertNotEqual(self.run_installer().returncode,0);self.assertEqual(dest.read_bytes(),b'previous model');self.assertEqual(self.calls.read_text(),'xxx');self.assertEqual(list(self.target.iterdir()),[dest])

if __name__=='__main__':unittest.main()
