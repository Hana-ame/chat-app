export default function ImagePreviewModal({ url, onClose }) {
  if (!url) return null;
  return (
    <div className="modal-overlay" onClick={onClose} style={{ cursor: 'zoom-out', background: 'rgba(0,0,0,0.85)' }}>
      <img src={url} style={{
        maxWidth: '90vw', maxHeight: '90vh', objectFit: 'contain',
        borderRadius: 8, display: 'block', margin: 'auto', position: 'absolute',
        top: '50%', left: '50%', transform: 'translate(-50%,-50%)',
      }} onClick={e => e.stopPropagation()} alt="" />
    </div>
  );
}
