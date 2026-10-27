"""Generate an explicitly synthetic receipt, without real financial information."""
from pathlib import Path
import argparse
import shutil
import subprocess

parser=argparse.ArgumentParser()
parser.add_argument('--number',default='DEMO-20261027-001')
parser.add_argument('--output-dir',type=Path,default=Path(__file__).parent)
args=parser.parse_args()
if not all(c.isalnum() or c=='-' for c in args.number):
    parser.error('Invoice number must use letters, digits and hyphens')

lines = ["TRAINING RECEIPT - NOT A TAX INVOICE", "Merchant: Demo Office Supplies", "Invoice No: "+args.number,
         "Invoice Date: 2026-10-27", "Currency: CNY", "Item: Office stationery", "Subtotal: 118.40", "Tax: 7.10", "TOTAL: CNY 125.50"]
stream = "BT /F1 16 Tf 55 730 Td " + " ".join(("0 -48 Td " if i else "")+"("+line+") Tj" for i, line in enumerate(lines)) + " ET"
objects = ["<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
           "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
           "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>", f"<< /Length {len(stream)} >>\nstream\n{stream}\nendstream"]
body = b"%PDF-1.4\n"
offsets = [0]
for i, item in enumerate(objects, 1):
    offsets.append(len(body))
    body += f"{i} 0 obj\n{item}\nendobj\n".encode()
start = len(body)
body += (f"xref\n0 {len(objects)+1}\n0000000000 65535 f \n" + "".join(f"{offset:010} 00000 n \n" for offset in offsets[1:]) +
         f"trailer << /Size {len(objects)+1} /Root 1 0 R >>\nstartxref\n{start}\n%%EOF\n").encode()
args.output_dir.mkdir(parents=True,exist_ok=True)
pdf=args.output_dir/'invoice-demo.pdf'
pdf.write_bytes(body)
subprocess.run(['pdftoppm','-f','1','-singlefile','-scale-to','1600','-png',str(pdf),str(args.output_dir/'invoice-demo')],check=True)
if args.output_dir.resolve()==Path(__file__).parent.resolve():
    public=Path(__file__).resolve().parents[3]/'web/public/samples'
    public.mkdir(parents=True,exist_ok=True)
    for extension in ('pdf','png'):
        shutil.copyfile(args.output_dir/('invoice-demo.'+extension),public/('invoice-demo.'+extension))
