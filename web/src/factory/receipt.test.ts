import {confirmed} from './receipt';
import type {Schema} from '../console/components';
test('无效批量创建回执不得显示成功',()=>{
 for(const value of [{},{agents:[]},{agents:[{id:'x'}]}])expect(()=>confirmed(value as Schema['FactoryResult'])).toThrow('结果尚未确认');
});
