// Subtle local contrast, a soft shoulder for daylight, and gentle desaturation
// in highlights. Uses only the displayed color so it works without depth access.
texture2D BackBuffer : COLOR;
sampler2D RoadColor { Texture = BackBuffer; };

void FullScreen(uint id : SV_VertexID, out float4 position : SV_Position, out float2 uv : TEXCOORD0)
{
    uv = float2((id << 1) & 2, id & 2);
    position = float4(uv * float2(2.0, -2.0) + float2(-1.0, 1.0), 0.0, 1.0);
}

float4 RoadFilm(float4 position : SV_Position, float2 uv : TEXCOORD0) : SV_Target
{
    const float2 step = float2(BUFFER_RCP_WIDTH, BUFFER_RCP_HEIGHT);
    const float3 weights = float3(0.2126, 0.7152, 0.0722);
    float3 color = tex2D(RoadColor, uv).rgb;
    float3 neighbors = (tex2D(RoadColor, uv + float2(step.x, 0.0)).rgb
                      + tex2D(RoadColor, uv - float2(step.x, 0.0)).rgb
                      + tex2D(RoadColor, uv + float2(0.0, step.y)).rgb
                      + tex2D(RoadColor, uv - float2(0.0, step.y)).rgb) * 0.25;
    float luma = dot(color, weights);
    float detail = clamp(dot(color - neighbors, weights), -0.045, 0.045);
    float mids = luma * (1.0 - luma);
    float graded = luma + 0.30 * detail + 0.22 * mids * (luma - 0.34);
    float3 tint = lerp(float3(-0.024, 0.0, 0.018), float3(0.020, 0.009, -0.016),
                       smoothstep(0.20, 0.78, luma)) * (4.0 * mids);
    color = (color - luma) * lerp(0.99, 0.91, smoothstep(0.58, 1.0, luma)) + graded + tint;
    return float4(saturate(color), 1.0);
}

technique OpenOMSIRoadFilm
{
    pass
    {
        VertexShader = FullScreen;
        PixelShader = RoadFilm;
    }
}
