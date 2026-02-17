import usehookData from './hook'
import moment from 'moment'

function Home() {
	const {timeObj, channels, selectedChannel, switchChannel, connectionState} = usehookData()
	
	// Get current channel info
	const currentChannel = channels.find(ch => ch.id === selectedChannel)
	
	// Connection state labels
	const connectionStateLabels = {
		0: 'Connecting...',
		1: 'Connected',
		2: 'Disconnecting...',
		3: 'Disconnected'
	}
	
	// Connection state colors
	const connectionStateColors = {
		0: 'text-highlight',
		1: 'text-cta',
		2: 'text-highlight',
		3: 'text-error-dark'
	}
	
	// Translucent card style (matching TPR.css .translucent-card-grey-1)
	const translucentCardStyle = {
		backgroundColor: 'rgba(31, 31, 27, 0.8)',
		boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
	}
	
	// Translucent blue card style (matching TPR.css .translucent-blue)
	const translucentBlueStyle = {
		backgroundColor: 'rgba(3, 68, 255, 0.5)',
		boxShadow: '2px 2px 1px rgba(0, 0, 0, 0.9)',
	}
	
	return (
		<div className='m-auto p-16' style={{ maxWidth: '600px' }}>
			{/* Title */}
			<div className='mb-12 text-17 font-bold text-platinum'>Video Channel</div>
			
			{/* Video Player (Camera Feed) - FIRST under title */}
			<div className='mb-16 flex justify-center'>
				<div 
					className='w-full rounded-xl overflow-hidden'
					style={{
						backgroundColor: 'rgba(31, 31, 27, 0.9)',
						boxShadow: '0 8px 32px rgba(0, 0, 0, 0.5)',
					}}
				>
					<video
						className='w-full'
						id='player'
						muted
						autoPlay
						playsInline
						style={{ aspectRatio: '16/9', objectFit: 'contain', background: '#1f1f1b' }}
					></video>
				</div>
			</div>
			
			{/* Channel Selector (Resolution Selection) */}
			<div className='flex gap-12 mb-16'>
				{channels.map(channel => (
					<button
						key={channel.id}
						onClick={() => switchChannel(channel.id)}
						className={`flex-1 py-14 px-16 rounded-lg transition-all duration-fast cursor-pointer border-0 ${
							selectedChannel === channel.id
								? 'text-white'
								: 'text-platinum hover:opacity-90'
						}`}
						style={selectedChannel === channel.id ? translucentBlueStyle : translucentCardStyle}
					>
						<div className='font-bold text-15'>CH{channel.id}</div>
						<div className='text-12 mt-4 opacity-80'>{channel.resolution}</div>
						<div className='text-11 mt-2 opacity-60'>{channel.fps} fps</div>
					</button>
				))}
			</div>
			
			{/* Resolution Information Card (with Time Stamp and Delay at bottom) */}
			{currentChannel && (
				<div
					className='p-16 rounded-lg'
					style={translucentCardStyle}
				>
					<div className='flex justify-between mb-8 py-4 border-b border-white/20'>
						<span className='text-platinum/70 text-14'>Resolution</span>
						<span className='font-medium text-platinum text-14'>{currentChannel.resolution}</span>
					</div>
					<div className='flex justify-between mb-8 py-4 border-b border-white/20'>
						<span className='text-platinum/70 text-14'>Frame Rate</span>
						<span className='font-medium text-platinum text-14'>{currentChannel.fps} fps</span>
					</div>
					<div className='flex justify-between mb-8 py-4 border-b border-white/20'>
						<span className='text-platinum/70 text-14'>Bitrate</span>
						<span className='font-medium text-platinum text-14'>{currentChannel.bitrate}</span>
					</div>
					<div className='flex justify-between mb-8 py-4 border-b border-white/20'>
						<span className='text-platinum/70 text-14'>Status</span>
						<span className={`font-semibold text-14 ${
							connectionStateColors[connectionState as keyof typeof connectionStateColors]
						}`}>
							{connectionStateLabels[connectionState as keyof typeof connectionStateLabels]}
						</span>
					</div>
					
					{/* Time Stamp and Delay - at bottom of resolution info card */}
					<div className='flex justify-between items-center pt-8 mt-8 border-t border-white/20'>
						<div>
							<div className='text-11 text-platinum/50 uppercase tracking-wide mb-4'>Time Stamp</div>
							<div className='text-14 font-medium text-cta'>
								{moment(timeObj.time||0).format('YYYY-MM-DD HH:mm:ss')}
							</div>
						</div>
						<div className='text-right'>
							<div className='text-11 text-platinum/50 uppercase tracking-wide mb-4'>Delay</div>
							<div className={`text-14 font-bold ${
								(timeObj.delay || 0) > 500 ? 'text-error' : 
								(timeObj.delay || 0) > 200 ? 'text-highlight' : 'text-cta'
							}`}>
								{timeObj.delay}ms
							</div>
						</div>
					</div>
				</div>
			)}
		</div>
	)
}

export default Home
