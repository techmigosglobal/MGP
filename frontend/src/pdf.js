import { jsPDF } from 'jspdf';
import autoTable from 'jspdf-autotable';

const label = (value) => String(value || '—').replaceAll('_', ' ');
const date = (value) => value ? new Intl.DateTimeFormat('en-IN', { dateStyle: 'medium' }).format(new Date(`${value}T00:00:00`)) : '—';

export function canProduceOfficialPdf(pass) {
  return ['APPROVED', 'PASSED_OUT', 'RETURNED'].includes(pass.status);
}

export function gatePassDocument(pass, organization = {}) {
  return {
    organization: organization.name || 'Material Gate Pass Management System',
    address: organization.address || 'Controlled material movement record',
    passNo: pass.pass_no,
    status: label(pass.status),
    fields: [
      ['Pass date', date(pass.pass_date)], ['Pass type', label(pass.pass_type)], ['Directorate / project', `${pass.directorate} / ${pass.project}`],
      ['Reference no.', pass.reference_no || '—'], ['Consignee', pass.consignee_name], ['Consignee address', pass.consignee_address || '—'],
      ['Purpose', pass.purpose], ['Authority', pass.authority], ['Vehicle no.', pass.vehicle_no || '—'],
      ['Packages', pass.packages], ['Expected return', date(pass.expected_return_date)], ['Actual return', date(pass.actual_return_date)],
      ['Security control', pass.security_control_no || '—'], ['Loaded in presence of', pass.loaded_in_presence_of || '—'], ['Carrier', [pass.carrier_name, pass.carrier_designation].filter(Boolean).join(' · ') || '—'],
    ],
    items: pass.items.map((item, index) => [index + 1, item.item_code || '—', item.item_name, item.serial_no || '—', item.quantity, item.unit_of_measure || 'NOS', item.description || '—']),
  };
}

export function downloadGatePassPdf(pass, organization) {
  const model = gatePassDocument(pass, organization);
  const doc = new jsPDF({ orientation: 'portrait', unit: 'mm', format: 'a4' });
  doc.setProperties({ title: `Material Gate Pass ${model.passNo}`, subject: 'Official material gate pass', author: model.organization });
  doc.setFillColor(14, 40, 58); doc.rect(0, 0, 210, 34, 'F');
  doc.setTextColor(255, 255, 255); doc.setFont('helvetica', 'bold'); doc.setFontSize(17); doc.text(model.organization, 14, 15);
  doc.setFont('helvetica', 'normal'); doc.setFontSize(8.5); doc.text(model.address, 14, 22);
  doc.setFont('helvetica', 'bold'); doc.setFontSize(13); doc.text('OFFICIAL MATERIAL GATE PASS', 196, 15, { align: 'right' });
  doc.setFont('helvetica', 'normal'); doc.setFontSize(9); doc.text(model.passNo, 196, 22, { align: 'right' });
  doc.setTextColor(24, 37, 49);
  doc.setFont('helvetica', 'bold'); doc.setFontSize(10); doc.text(`STATUS: ${model.status}`, 14, 43);
  autoTable(doc, { startY: 48, theme: 'grid', styles: { fontSize: 8, cellPadding: 2.2, lineColor: [199, 210, 219], lineWidth: 0.2 }, columnStyles: { 0: { fontStyle: 'bold', fillColor: [238, 244, 247], cellWidth: 31 }, 1: { cellWidth: 64 }, 2: { fontStyle: 'bold', fillColor: [238, 244, 247], cellWidth: 31 }, 3: { cellWidth: 64 } }, body: model.fields.reduce((rows, field, index) => index % 2 === 0 ? [...rows, [field[0], field[1], model.fields[index + 1]?.[0] || '', model.fields[index + 1]?.[1] || '']] : rows, []) });
  const materialY = doc.lastAutoTable.finalY + 8;
  doc.setFont('helvetica', 'bold'); doc.setFontSize(10); doc.text('MATERIAL DETAILS', 14, materialY);
  autoTable(doc, { startY: materialY + 4, theme: 'grid', head: [['#', 'Code', 'Material', 'Serial / batch', 'Qty', 'Unit', 'Description']], body: model.items, styles: { fontSize: 7.5, cellPadding: 2, lineColor: [199, 210, 219], lineWidth: 0.2 }, headStyles: { fillColor: [14, 40, 58] }, columnStyles: { 0: { cellWidth: 8 }, 1: { cellWidth: 21 }, 3: { cellWidth: 28 }, 4: { halign: 'right', cellWidth: 12 }, 5: { cellWidth: 12 } } });
  let signatureY = doc.lastAutoTable.finalY + 16;
  if (signatureY > 265) { doc.addPage(); signatureY = 28; }
  doc.setDrawColor(91, 107, 121); doc.line(14, signatureY, 68, signatureY); doc.line(78, signatureY, 132, signatureY); doc.line(142, signatureY, 196, signatureY);
  doc.setFont('helvetica', 'bold'); doc.setFontSize(8); doc.text('Prepared / Inventory', 14, signatureY + 5); doc.text('Approved / Issuing', 78, signatureY + 5); doc.text('Security / Return', 142, signatureY + 5);
  doc.setFont('helvetica', 'normal'); doc.setFontSize(7); doc.text(`Generated ${new Date().toLocaleString('en-IN')} · Backend-authoritative record`, 14, 287);
  doc.save(`MGP-${pass.pass_no.replaceAll('/', '-')}.pdf`);
}
