import UserAvatar from './UserAvatar';

export default function MemberList({ members, chat, currentUserId, onProfile, onKick }) {
  const isAdmin = m => m.role === 'admin' || m.id === chat?.owner_id;

  return members.map(m => (
    <div key={m.id} style={{display:'flex',alignItems:'center',gap:8,padding:'4px 0',fontSize:14,cursor:'pointer'}}
      onClick={() => onProfile?.(m)}>
      <span className={'status-dot ' + (m.isOnline ? 'online' : 'offline')} style={{flexShrink:0}} />
      <UserAvatar user={m} size={28} />
      <span style={{overflow:'hidden',textOverflow:'ellipsis',whiteSpace:'nowrap',minWidth:0}}>{m.username}</span>
      <div style={{flex:1}} />
      <div style={{width:66,height:28,position:'relative',flexShrink:0}}>
        {isAdmin(m) && <span style={{position:'absolute',right:onKick && chat?.owner_id === currentUserId && m.id !== currentUserId && chat?.type !== 'dm' ? 22 : 0,top:'50%',transform:'translateY(-50%)',fontSize:10,padding:'0 5px',borderRadius:3,fontWeight:500,background:'var(--accent-bg)',color:'var(--accent)'}}>ADMIN</span>}
        {onKick && chat?.owner_id === currentUserId && m.id !== currentUserId && chat?.type !== 'dm' && (
          <button className="btn-ghost" style={{position:'absolute',right:0,top:'50%',transform:'translateY(-50%)',fontSize:12}} onClick={(e) => { e.stopPropagation(); onKick(m.id); }}>×</button>
        )}
      </div>
    </div>
  ));
}
