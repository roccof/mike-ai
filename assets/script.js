const ws = new WebSocket("ws://127.0.0.1:8080/ws");
const wavRecorder = new WavRecorder({ sampleRate: 24000 });
const wavStreamPlayer = new WavStreamPlayer({ sampleRate: 24000 });
let canvas;
let ctx;

window.onload = function() {
	canvas = document.getElementById("paintbox");
	ctx = canvas.getContext("2d");
}

ws.onopen = async () => {
	console.log("WebSocket connection established.");

	await wavRecorder.begin();
	await wavStreamPlayer.connect();

	await wavRecorder.record((data) => {
		const { mono, raw } = data;
		ws.send(JSON.stringify({
			type: "audio",
			audio: arrayBufferToBase64(mono),
		}));
	});
};

ws.onmessage = async (event) => {
	msg = JSON.parse(event.data);

	if (msg.type == "audio") {
		wavStreamPlayer.add16BitPCM(base64ToArrayBuffer(msg.audio), "ai-audio");
	} else if (msg.type == "command.invoke") {
		const fn = cmds[msg.name];
		if (fn !== undefined) {
			console.log(`Calling local function ${msg.name} with ${msg.args}`);
			const args = JSON.parse(msg.args);
			const result = await fn(args);
			console.log('result', result);
			ws.send(JSON.stringify({
				type: "command.result",
				call_id: msg.call_id,
				name: msg.name,
				output: JSON.stringify(result),
			}));
		} else {
			console.log(`Local function ${msg.name} not found`);
			ws.send(JSON.stringify({
				type: "command.result",
				call_id: msg.call_id,
				name: msg.name,
				output: JSON.stringify({success: false, error: "not found"}),
			}));
		}

	} else {
		console.log("Received unsupported message: ", msg);
	}
};

ws.onerror = async (err) => {
	console.error("WebSocket error: ", err);
};

ws.onclose = async () => {
	console.log("WebSocket connection closed.");
	await wavRecorder.pause();
	await wavRecorder.end();
	await wavStreamPlayer.interrupt();
};

/*
 * Copied from: 
 * 	https://github.com/openai/openai-realtime-api-beta/blob/main/lib/utils.js#L30
 */
function base64ToArrayBuffer(base64) {
	const binaryString = atob(base64);
	const len = binaryString.length;
	const bytes = new Uint8Array(len);
	for (let i = 0; i < len; i++) {
		bytes[i] = binaryString.charCodeAt(i);
	}
	return bytes.buffer;
}

/*
 * Copied from:
 * 	https://github.com/openai/openai-realtime-api-beta/blob/main/lib/utils.js#L14
 */
function floatTo16BitPCM(float32Array) {
	const buffer = new ArrayBuffer(float32Array.length * 2);
	const view = new DataView(buffer);
	let offset = 0;
	for (let i = 0; i < float32Array.length; i++, offset += 2) {
		let s = Math.max(-1, Math.min(1, float32Array[i]));
		view.setInt16(offset, s < 0 ? s * 0x8000 : s * 0x7fff, true);
	}
	return buffer;
}

/*
 * Copied from:
 * 	https://github.com/openai/openai-realtime-api-beta/blob/main/lib/utils.js#L45
 */
function arrayBufferToBase64(arrayBuffer) {
	if (arrayBuffer instanceof Float32Array) {
		arrayBuffer = floatTo16BitPCM(arrayBuffer);
	} else if (arrayBuffer instanceof Int16Array) {
		arrayBuffer = arrayBuffer.buffer;
	}
	let binary = '';
	let bytes = new Uint8Array(arrayBuffer);
	const chunkSize = 0x8000; // 32KB chunk size
	for (let i = 0; i < bytes.length; i += chunkSize) {
		let chunk = bytes.subarray(i, i + chunkSize);
		binary += String.fromCharCode.apply(null, chunk);
	}
	return btoa(binary);
}

const cmds = {
	getCanvasSize: () => {
		return { width: canvas.width, height: canvas.height };
	},
	clearCanvas: () => {
		ctx.clearRect(0, 0, canvas.width, canvas.height);
		return { success: true };
	},
	paintCanvas: async (commands) => {
		console.log("Received paintCanvas command with commands:", commands);
		commands.commands.forEach(async (cmd) => {
			const fn = fns[cmd.command];
			if (fn !== undefined) {
				console.log(`Invoking command ${cmd.command} with ${cmd.params}`);
				result =  await fn(cmd.params);
				if (result.success == false) {
					console.log(`Command ${cmd.command} failed`);
					return { success: false, error: "command failed" };
				}
			} else {
				console.log(`Command ${cmd.command} not found`);
				return { success: false, error: "command not found" };
			}
		});

		return { success: true, commands};
	},
}

const fns = {
	fillRect: ({ x, y, width, height }) => {
		ctx.fillRect(x, y, width, height);
		return { success: true, x, y, width, height };
	},
	strokeRect: ({ x, y, width, height }) => {
		ctx.strokeRect(x, y, width, height);
		return { success: true, x, y, width, height };
	},
	clearRect: ({ x, y, width, height }) => {
		ctx.clearRect(x, y, width, height);
		return { success: true, x, y, width, height };
	},
	beginPath: () => {
		ctx.beginPath();
		return { success: true };
	},
	closePath: () => {
		ctx.closePath();
		return { success: true };
	},
	stroke: () => {
		ctx.stroke();
		return { success: true };
	},
	fill: () => {
		ctx.fill();
		return { success: true };
	},
	moveTo: ({ x, y }) => {
		ctx.moveTo(x, y);
		return { success: true, x, y };
	},
	lineTo: ({ x, y }) => {
		ctx.lineTo(x, y);
		return { success: true, x, y };
	},
	arc: ({ x, y, radius, startAngle, endAngle, counterclockwise }) => {
		ctx.arc(x, y, radius, startAngle, endAngle, counterclockwise);
		return { success: true, x, y, radius, startAngle, endAngle, counterclockwise };
	},
	arcTo: ({ x1, y1, x2, y2, radius }) => {
		ctx.arcTo(x1, y1, x2, y2, radius);
		return { success: true, x1, y1, x2, y2, radius };
	},
	quadraticCurveTo: ({ cp1x, cp1y, x, y }) => {
		ctx.quadraticCurveTo(cp1x, cp1y, x, y);
		return { success: true, cp1x, cp1y, x, y };
	},
	bezierCurveTo: ({ cp1x, cp1y, cp2x, cp2y, x, y }) => {
		ctx.bezierCurveTo(cp1x, cp1y, cp2x, cp2y, x, y);
		return { success: true, cp1x, cp1y, cp2x, cp2y, x, y };
	},
	rect: ({ x, y, width, height }) => {
		ctx.rect(x, y, width, height);
		return { success: true, x, y, width, height };
	},
	setFillStyle: ({ color }) => {
		ctx.fillStyle = color;
		return { success: true, color };
	},
	setStrokeStyle: ({ color }) => {
		ctx.strokeStyle = color;
		return { success: true, color };
	},
	setGlobalAlpha: ({ alpha }) => {
		ctx.globalAlpha = alpha;
		return { success: true, alpha };
	},
	setLineWidth: ({ width }) => {
		ctx.lineWidth = width;
		return { success: true, width: ctx.lineWidth };
	},
	setLineCap: ({ cap }) => {
		ctx.lineCap = cap;
		return { success: true, lineCap: ctx.lineCap };
	},
	setLineJoin: ({ join }) => {
		ctx.lineJoin = join;
		return { success: true, lineJoin: ctx.lineJoin };
	},
	setMiterLimit: ({ limit }) => {
		ctx.miterLimit = limit;
		return { success: true, miterLimit: ctx.miterLimit };
	},
	setShadowOffsetX: ({ offsetX }) => {
		ctx.shadowOffsetX = offsetX;
		return { success: true, offsetX };
	},
	setShadowOffsetY: ({ offsetY }) => {
		ctx.shadowOffsetY = offsetY;
		return { success: true, offsetY };
	},
	setShadowBlur: ({ blur }) => {
		ctx.shadowBlur = blur;
		return { success: true, blur };
	},
	setShadowColor: ({ color }) => {
		ctx.shadowColor = color;
		return { success: true, color };
	},
	fillText: ({ text, x, y }) => {
		ctx.fillText(text, x, y);
		return { success: true, text, x, y };
	},
	strokeText: ({ text, x, y }) => {
		ctx.strokeText(text, x, y);
		return { success: true, text, x, y };
	},
	setFont: ({ font }) => {
		ctx.font = font;
		return { success: true, font };
	},
	setTextAlign: ({ align }) => {
		ctx.textAlign = align;
		return { success: true, align };
	},
	setTextBaseline: ({ baseline }) => {
		ctx.textBaseline = baseline;
		return { success: true, baseline };
	},
	setTextDirection: ({ direction }) => {
		ctx.direction = direction;
		return { success: true, direction };
	},
};
